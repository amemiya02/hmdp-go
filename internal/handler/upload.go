package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

// UploadBlogImage 上传探店笔记图片 (POST /upload/blog)
func (h *UploadHandler) UploadBlogImage(c *gin.Context) {
	// 1. 获取上传的文件 (@RequestParam("file") MultipartFile image)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail("上传文件失败: "+err.Error()))
		return
	}

	// 2. 获取原始文件名称并提取后缀
	originalFilename := file.Filename
	ext := filepath.Ext(originalFilename) // filepath.Ext 会连带 . 一起获取，比如 ".jpg"

	// 3. 生成新文件名和目录
	fileName, destPath, err := createNewFileName(ext)
	if err != nil {
		global.Logger.Errorf("生成文件路径失败: %v", err)
		c.JSON(http.StatusOK, dto.Fail("系统繁忙，文件路径生成失败"))
		return
	}

	// 4. 保存文件到本地磁盘 (对应 image.transferTo(...))
	if err := c.SaveUploadedFile(file, destPath); err != nil {
		global.Logger.Errorf("文件保存失败: %v", err)
		c.JSON(http.StatusOK, dto.Fail("文件保存失败"))
		return
	}

	// 5. 返回结果 (返回相对路径给前端)
	global.Logger.Debugf("文件上传成功，%s", fileName)
	c.JSON(http.StatusOK, dto.OkWithData(fileName))
}

// DeleteBlogImage 删除探店笔记图片 (对应 GET /upload/blog/delete)
func (h *UploadHandler) DeleteBlogImage(c *gin.Context) {
	// 获取文件名 (对应 @RequestParam("name") String filename)
	filename := c.Query("name")
	if filename == "" {
		c.JSON(http.StatusOK, dto.Fail("文件名不能为空"))
		return
	}

	// 拼接完整的物理路径
	fullPath := filepath.Join(constant.ImageUploadDir, filename)

	// 判断是否是目录 (防范恶意参数)
	info, err := os.Stat(fullPath)
	if err == nil && info.IsDir() {
		c.JSON(http.StatusOK, dto.Fail("错误的文件名称"))
		return
	}

	// 删除文件 (对应 FileUtil.del(file))
	err = os.Remove(fullPath)
	// 如果错误不是"文件不存在"（说明真报错了），则记录日志
	if err != nil && !os.IsNotExist(err) {
		global.Logger.Errorf("删除文件失败: %v", err)
		c.JSON(http.StatusOK, dto.Fail("文件删除失败"))
		return
	}

	c.JSON(http.StatusOK, dto.Ok())
}

// createNewFileName 生成打散的目录和文件名
func createNewFileName(ext string) (string, string, error) {
	// 1. 生成不带横线的 UUID
	name := strings.ReplaceAll(uuid.New().String(), "-", "")

	// 2. 目录打散算法（Go 优化版）
	// 取 UUID 的第1个字符和第2个字符作为两级目录（即 16 * 16 = 256 个子目录）
	// 完全等价于 Java 版中 hashCode() & 0xF 的散列效果
	d1 := name[0:1]
	d2 := name[1:2]

	// 3. 相对路径 (用于返回给前端，存入数据库)
	// 格式例如: /blogs/a/f/af123456...789.jpg
	relativePath := fmt.Sprintf("/blogs/%s/%s/%s%s", d1, d2, name, ext)

	// 4. 物理/绝对路径 (用于磁盘写入)
	dirPath := filepath.Join(constant.ImageUploadDir, "blogs", d1, d2)

	// 5. 判断目录是否存在，不存在则创建 (对应 dir.mkdirs())
	// os.ModePerm 代表 0777 权限
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return "", "", err
	}

	// 6. 最终的磁盘落子路径
	destPath := filepath.Join(dirPath, name+ext)

	return relativePath, destPath, nil
}
