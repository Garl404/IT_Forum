package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/middleware"
	"github.com/Garl404/IT_Forum/backend/internal/models"
)

var blockedExts = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".scr": true, ".ps1": true,
	".vbs": true, ".js": true, ".jse": true, ".wsf": true, ".msi": true, ".dll": true,
	".sh": true, ".py": true, ".rb": true, ".pl": true, ".php": true, ".jsp": true,
	".lnk": true, ".hta": true, ".cpl": true, ".jar": true, ".reg": true, ".pif": true,
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// Upload 文件上传：配额 = 基础额度(5GB，10级15GB) + 误判补偿
func Upload(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonErr(w, 400, "bad_request", "表单解析失败")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, 400, "no_file", "未选择文件")
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		jsonErr(w, 400, "invalid", "文件为空")
		return
	}
	if header.Size > config.Cfg.Content.Upload.MaxFileBytes {
		jsonErr(w, 413, "too_large", fmt.Sprintf("单文件最大 %.0f GB", float64(config.Cfg.Content.Upload.MaxFileBytes)/(1<<30)))
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if blockedExts[ext] {
		jsonErr(w, 400, "blocked_ext", "该文件类型不允许上传")
		return
	}

	quota := uploadQuota(u)
	if u.UploadUsedBytes+header.Size > quota {
		jsonErr(w, 413, "quota_exceeded", fmt.Sprintf("上传配额不足（已用 %.2f GB / 共 %.2f GB）",
			float64(u.UploadUsedBytes)/(1<<30), float64(quota)/(1<<30)))
		return
	}

	fname := sanitizeFilename(header.Filename)
	stored := randName() + ext
	dir := config.Cfg.Content.Upload.Dir
	if err := os.MkdirAll(dir, 0o750); err != nil {
		jsonErr(w, 500, "internal", "存储目录错误")
		return
	}
	dst, err := os.OpenFile(filepath.Join(dir, stored), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		jsonErr(w, 500, "internal", "保存文件失败")
		return
	}
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(dst, hasher), file)
	dst.Close()
	if err != nil {
		os.Remove(filepath.Join(dir, stored))
		jsonErr(w, 500, "internal", "写入失败")
		return
	}
	if size == 0 {
		os.Remove(filepath.Join(dir, stored))
		jsonErr(w, 400, "invalid", "文件为空")
		return
	}
	// 收紧：按实际写入大小再次校验配额
	var used int64
	database.DB.QueryRow(`SELECT COALESCE(SUM(size),0) FROM uploads WHERE user_id=?`, u.ID).Scan(&used)
	if used+size > quota {
		os.Remove(filepath.Join(dir, stored))
		jsonErr(w, 413, "quota_exceeded", "上传配额不足")
		return
	}

	postID := int64(0)
	if v := r.FormValue("post_id"); v != "" {
		postID, _ = strconv.ParseInt(v, 10, 64)
		var owner int64
		if err := database.DB.QueryRow(`SELECT user_id FROM posts WHERE id=?`, postID).Scan(&owner); err != nil || owner != u.ID {
			postID = 0
		}
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" || len(mime) > 100 {
		mime = "application/octet-stream"
	}
	out, err := database.DB.Exec(`INSERT INTO uploads(user_id, post_id, filename, stored_name, size, mime, sha256, created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		u.ID, postID, fname, stored, size, mime, hex.EncodeToString(hasher.Sum(nil)), time.Now().Unix())
	if err != nil {
		os.Remove(filepath.Join(dir, stored))
		jsonErr(w, 500, "internal", "记录失败")
		return
	}
	id, _ := out.LastInsertId()
	jsonCreated(w, map[string]interface{}{"id": id, "filename": fname, "size": size, "post_id": postID})
}

func randName() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func MyUploads(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	rows, err := database.DB.Query(`SELECT id, user_id, post_id, filename, size, mime, sha256, created_at
		FROM uploads WHERE user_id=? ORDER BY created_at DESC LIMIT 200`, u.ID)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []models.Upload{}
	for rows.Next() {
		var up models.Upload
		if rows.Scan(&up.ID, &up.UserID, &up.PostID, &up.Filename, &up.Size, &up.Mime, &up.Sha256, &up.CreatedAt) == nil {
			list = append(list, up)
		}
	}
	jsonOK(w, map[string]interface{}{"uploads": list})
}

// Download 下载（仅文件所有者或管理员）
func Download(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	u := middleware.UserFrom(r.Context())
	var up models.Upload
	err = database.DB.QueryRow(`SELECT id, user_id, filename, stored_name, size, mime FROM uploads WHERE id=?`, id).Scan(
		&up.ID, &up.UserID, &up.Filename, &up.StoredName, &up.Size, &up.Mime)
	if err != nil {
		jsonErr(w, 404, "not_found", "文件不存在")
		return
	}
	if u == nil || (u.ID != up.UserID && u.Role != models.RoleAdmin) {
		jsonErr(w, 403, "forbidden", "无权下载")
		return
	}
	path := filepath.Join(config.Cfg.Content.Upload.Dir, up.StoredName)
	// 防路径穿越
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(config.Cfg.Content.Upload.Dir)) {
		jsonErr(w, 403, "forbidden", "非法路径")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		jsonErr(w, 404, "not_found", "文件已丢失")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, urlEncode(up.Filename)))
	w.Header().Set("Content-Length", strconv.FormatInt(up.Size, 10))
	io.Copy(w, f)
}

func urlEncode(s string) string {
	var b strings.Builder
	for _, r := range []byte(s) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '~' {
			b.WriteByte(r)
		} else {
			fmt.Fprintf(&b, "%%%02X", r)
		}
	}
	return b.String()
}

func DeleteUpload(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	u := middleware.UserFrom(r.Context())
	var up models.Upload
	err = database.DB.QueryRow(`SELECT id, user_id, stored_name FROM uploads WHERE id=?`, id).Scan(&up.ID, &up.UserID, &up.StoredName)
	if err != nil {
		jsonErr(w, 404, "not_found", "文件不存在")
		return
	}
	if u.ID != up.UserID && u.Role != models.RoleAdmin {
		jsonErr(w, 403, "forbidden", "无权删除")
		return
	}
	database.DB.Exec(`DELETE FROM uploads WHERE id=?`, id)
	os.Remove(filepath.Join(config.Cfg.Content.Upload.Dir, up.StoredName))
	jsonOK(w, map[string]string{"ok": "已删除"})
}
