package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/shogo82148/androidbinary/apk"

	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/models"
)

// FileSHA256 returns the hex-encoded SHA-256 of the file at path,
// or an empty string if the file is missing/unreadable.
func FileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// APKMeta extracted from AndroidManifest.xml inside the APK.
// All fields are best-effort; missing values come back empty.
type APKMeta struct {
	PackageName string
	VersionName string
	VersionCode int32
	AppLabel    string
	MinSDK      int32
	TargetSDK   int32
}

// hardcoded labels for known apps so we never end up displaying a
// non-Latin localization that the parser happened to pick first.
var knownLabels = map[string]string{
	"com.whatsapp":     "WhatsApp Messenger",
	"com.whatsapp.w4b": "WhatsApp Business",
}

func friendlyLabel(packageName, parsedLabel string) string {
	if v, ok := knownLabels[packageName]; ok {
		return v
	}
	// Crude Latin check: if more than half the characters are basic ASCII, accept.
	if parsedLabel != "" && looksLatin(parsedLabel) {
		return parsedLabel
	}
	return packageName
}

func looksLatin(s string) bool {
	if s == "" {
		return false
	}
	ascii := 0
	total := 0
	for _, r := range s {
		total++
		if r < 0x80 {
			ascii++
		}
	}
	return total > 0 && ascii*2 >= total
}

func ParseAPKMeta(path string) (APKMeta, error) {
	var m APKMeta
	pkg, err := apk.OpenFile(path)
	if err != nil {
		return m, err
	}
	defer pkg.Close()

	mf := pkg.Manifest()
	m.PackageName = mf.Package.MustString()
	m.VersionName = mf.VersionName.MustString()
	m.VersionCode = mf.VersionCode.MustInt32()
	m.MinSDK = mf.SDK.Min.MustInt32()
	m.TargetSDK = mf.SDK.Target.MustInt32()
	parsed := ""
	if label, err := pkg.Label(nil); err == nil {
		parsed = label
	}
	m.AppLabel = friendlyLabel(m.PackageName, parsed)
	return m, nil
}

// FixupKnownAPKLabels normalises app_label for any APK rows whose
// package_name has a hardcoded friendly label. Cheap to run on startup.
func FixupKnownAPKLabels(database *sql.DB) {
	for pkg, label := range knownLabels {
		_, err := database.Exec(
			`UPDATE apks SET app_label = $1 WHERE package_name = $2 AND app_label <> $1`,
			label, pkg,
		)
		if err != nil {
			log.Printf("FixupKnownAPKLabels: %v", err)
		}
	}
}

// APKDir returns the directory portion of WHATSAPP_APK_PATH where every APK
// (indexed by UUID) is stored.
func APKDir(legacyAPKPath string) string {
	return filepath.Dir(legacyAPKPath)
}

// APKFilePath returns the on-disk path for a given APK record.
func APKFilePath(apkDir string, a *models.APK) string {
	return filepath.Join(apkDir, a.Filename)
}

// IngestAPKFile imports an APK file from the given source path into the
// managed library, returning the inserted record. The source file is
// moved (renamed) to <apkDir>/<uuid>.apk on success.
func IngestAPKFile(database *sql.DB, srcPath, apkDir, source string, makeDefault bool) (*models.APK, error) {
	st, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	hash := FileSHA256(srcPath)
	if hash == "" {
		return nil, errors.New("failed to compute hash")
	}
	if existing, _ := db.GetAPKBySHA256(database, hash); existing != nil {
		// Same APK already known. If the source is the legacy single-file path,
		// remove it so we don't carry two copies on disk.
		if srcPath != APKFilePath(apkDir, existing) {
			_ = os.Remove(srcPath)
		}
		return existing, nil
	}
	meta, err := ParseAPKMeta(srcPath)
	if err != nil {
		return nil, fmt.Errorf("parse apk: %w", err)
	}
	a := &models.APK{
		Filename:    "", // assigned below from inserted id
		PackageName: meta.PackageName,
		VersionName: meta.VersionName,
		VersionCode: int64(meta.VersionCode),
		AppLabel:    meta.AppLabel,
		SizeBytes:   st.Size(),
		SHA256:      hash,
		MinSDK:      int(meta.MinSDK),
		TargetSDK:   int(meta.TargetSDK),
		IsDefault:   false, // updated below if requested
		Source:      source,
	}
	// First insert (filename TBD; we use uuid then patch by renaming)
	a.Filename = "_pending_.apk"
	inserted, err := db.InsertAPK(database, a)
	if err != nil {
		return nil, fmt.Errorf("insert apk row: %w", err)
	}
	finalName := inserted.ID + ".apk"
	finalPath := filepath.Join(apkDir, finalName)

	if srcPath != finalPath {
		if err := os.Rename(srcPath, finalPath); err != nil {
			// best-effort copy as fallback (cross-fs)
			if cerr := copyFile(srcPath, finalPath); cerr != nil {
				return nil, fmt.Errorf("place file: %w (rename: %v)", cerr, err)
			}
			_ = os.Remove(srcPath)
		}
	}
	if _, err := database.Exec(`UPDATE apks SET filename = $1 WHERE id = $2`, finalName, inserted.ID); err != nil {
		return nil, fmt.Errorf("update filename: %w", err)
	}
	inserted.Filename = finalName

	if makeDefault {
		if err := db.SetDefaultAPK(database, inserted.ID); err != nil {
			log.Printf("warn: failed to set default APK: %v", err)
		} else {
			inserted.IsDefault = true
		}
	}

	// Auto-link existing devices that already had this exact APK installed
	if linked, err := db.LinkDevicesByHash(database, hash, inserted.ID); err == nil && linked > 0 {
		log.Printf("ingestAPK: linked %d existing device(s) by sha256 to apk %s", linked, inserted.ID)
	}
	return inserted, nil
}

// IngestLegacyAPK runs once on startup. If the legacy single-file APK exists
// at WHATSAPP_APK_PATH and is not yet tracked in the apks table, imports it
// and marks it default (so existing devices keep working).
func IngestLegacyAPK(database *sql.DB, legacyAPKPath string) {
	if _, err := os.Stat(legacyAPKPath); err != nil {
		return
	}
	any, err := db.ListAPKs(database)
	if err != nil {
		log.Printf("legacy ingest: list apks failed: %v", err)
		return
	}
	makeDefault := len(any) == 0
	rec, err := IngestAPKFile(database, legacyAPKPath, APKDir(legacyAPKPath), "legacy", makeDefault)
	if err != nil {
		log.Printf("legacy ingest skipped: %v", err)
		return
	}
	log.Printf("legacy ingest OK: id=%s version=%s default=%t", rec.ID, rec.VersionName, rec.IsDefault)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
