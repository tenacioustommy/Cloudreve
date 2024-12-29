package sync

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Sync 同步本地目录的文件到数据库
func Sync() {
	model.Init()
	// 获取所有用户
	users, err := model.GetAllUsers()
	if err != nil {
		util.Log().Error("获取用户时出错: %v", err)
		return
	}

	// 获取每个用户对应的组及其策略
	for _, user := range users {
		policy, err := model.GetPolicyByID(user.GetPolicyID(0))
		if err != nil {
			util.Log().Error("获取用户 %s 的策略时出错: %v", user.Email, err)
			continue
		}
		user.Policy = policy
		// 调用单独的同步函数
		if err := syncUserFiles(&user); err != nil {
			util.Log().Error("同步用户 %s 的文件时出错: %v", user.Email, err)
		}
	}
}
func uploadFile(fs *filesystem.FileSystem, base_dir string, path string) error {
	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		util.Log().Error("获取文件信息 %s 时出错: %v", path, err)
		return err
	}
	relPath, err := filepath.Rel(base_dir, filepath.Dir(path))
	if err != nil {
		return err
	}
	if relPath == "." {
		relPath = ""
	}
	file := &fsctx.FileStream{
		Mode:        fsctx.Nop,
		Size:        uint64(info.Size()),
		Name:        filepath.Base(path),
		VirtualPath: "/" + filepath.ToSlash(relPath),
		File:        ioutil.NopCloser(strings.NewReader("")),
		MimeType:    util.GetMimeType(path),
		SavePath:    path,
	}
	return filesystem.GenericAfterUpload(nil, fs, file)

}

// SyncUserFiles 同步单个用户的文件到数据库
func syncUserFiles(user *model.User) error {
	// 假设 filesystem.NewFileSystem 需要 *model.User
	filsys, err := filesystem.NewFileSystem(user)
	if err != nil {
		util.Log().Error("创建文件系统时出错: %v", err)
		return err
	}

	base_dir := filsys.Policy.GeneratePath(
		user.Model.ID,
		"/",
	)

	if _, err := os.Stat(base_dir); os.IsNotExist(err) {
		util.Log().Info("基础目录 %s 不存在，跳过同步", base_dir)
		return nil
	}

	files, err := model.GetFilesByUserID(user.Model.ID)
	if err != nil {
		util.Log().Error("获取用户 %s 的文件时出错: %v", user.Email, err)
		return err
	}
	// folderNum := make(map[uint]int)
	// 只用一个fileToDelete，key为文件路径，value为文件ID
	fileToDelete := make(map[string]model.File)
	for _, file := range files {
		fileToDelete[file.SourceName] = file
		// if _, ok := folderNum[file.FolderID]; !ok {
		// 	folderNum[file.FolderID] = 1
		// } else {
		// 	folderNum[file.FolderID]++
		// }

	}

	// 开始同步文件
	// 添加只有本地存在的文件
	err = filepath.WalkDir(base_dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.Contains(d.Name(), ".cache") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_thumb") {
			return nil
		}
		path = filepath.ToSlash(path)
		if _, ok := fileToDelete[path]; !ok {
			// 不在fileToDelete中，说明是新文件，上传到数据库
			uploadFile(filsys, base_dir, path)
		} else {
			// 数据库中有，本地也有，删除fileToDelete中的记录
			delete(fileToDelete, path)
		}
		return nil
	})
	if err != nil {
		util.Log().Error("遍历目录时出错: %v", err)
		return err
	}
	// 将fileToDelete的value转换为list
	var fileList []uint
	for _, file := range fileToDelete {
		fileList = append(fileList, file.Model.ID)
		// folderNum[file.FolderID]--
	}
	// var folderList []uint
	// for folderId, num := range folderNum {
	// 	if num == 0 {
	// 		folderList = append(folderList, folderId)
	// 	}
	// }

	if len(fileList) > 0 {
		filsys.Delete(nil, nil, fileList, false, true)
	}
	// if len(folderList) > 0 {
	// 	filsys.Delete(nil, folderList, nil, false, true)
	// }

	return nil
}
