/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package skill

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/bkaidev"
	"hcm/pkg/tools/localstore"
)

// defaultDirPerm is the permission used when creating directories.
const defaultDirPerm = 0755

// skillRecord is the local version record for a single installed skill.
type skillRecord struct {
	// Version is the version string from the remote platform.
	Version string `json:"version"`
	// MD5 is the optional checksum of the skill archive.
	// TODO: 目前bkaidev skill列表接口没有返回md5，后续提供后再补充，目前也是通过version管理的版本
	MD5 string `json:"md5,omitempty"`
	// Tags is the tag list returned by BKAIDev, each inner slice is [tag_id, tag_name].
	Tags [][]string `json:"tags,omitempty"`
	// UpdatedAt is the timestamp of the last successful install.
	UpdatedAt time.Time `json:"updated_at"`
}

// Installer downloads, extracts, and registers a single skill from BKAIDev.
// The install flow: get download URL → download zip to archiveDir →
// extract to a temp dir under root → atomic rename → update local store.
type Installer struct {
	client     bkaidev.Client
	store      *localstore.Store[skillRecord]
	skillRoot  string
	archiveDir string
}

// newInstaller creates an Installer.
func newInstaller(cli bkaidev.Client, store *localstore.Store[skillRecord], skillRoot, archiveDir string) *Installer {
	return &Installer{
		client:     cli,
		store:      store,
		skillRoot:  skillRoot,
		archiveDir: archiveDir,
	}
}

// Install downloads the skill zip for item and extracts it to skillRoot/{installKey}/.
// On success it updates the manifest entry. On failure the manifest is not
// modified so the next sync can retry.
func (inst *Installer) Install(kt *kit.Kit, item bkaidev.SkillListItem) error {
	key := item.InstallKey()
	version := item.Version

	urlResult, err := inst.client.GetSkillDownloadURL(kt, &bkaidev.GetSkillDownloadURLReq{SkillID: item.ID})
	if err != nil {
		logs.Errorf("get skill download url %s@%s failed, err: %v, rid: %s", key, version, err, kt.Rid)
		return fmt.Errorf("get download url for skill %s@%s: %w", key, version, err)
	}

	zipPath, err := inst.downloadZip(kt, key, version, urlResult.URL)
	if err != nil {
		logs.Errorf("download skill %s@%s failed, err: %v, rid: %s", key, version, err, kt.Rid)
		return fmt.Errorf("download skill %s@%s: %w", key, version, err)
	}

	if err = inst.extract(kt, zipPath, key); err != nil {
		logs.Errorf("extract skill %s@%s failed, err: %v, rid: %s", key, version, err, kt.Rid)
		return fmt.Errorf("extract skill %s@%s: %w", key, version, err)
	}

	entry := skillRecord{
		Version:   version,
		Tags:      item.TagNames,
		UpdatedAt: time.Now(),
	}
	if err = inst.store.Set(key, entry); err != nil {
		logs.Errorf("update local store for skill %s failed, err: %v, rid: %s", key, err, kt.Rid)
		return fmt.Errorf("update local store for skill %s: %w", key, err)
	}

	logs.Infof("skill installed: key=%s skill_id=%d version=%s, rid: %s", key, item.ID, version, kt.Rid)
	return nil
}

// downloadZip fetches the zip archive to archiveDir/{key}/{version}.zip.
func (inst *Installer) downloadZip(kt *kit.Kit, key, version, downloadURL string) (string, error) {
	dir := filepath.Join(inst.archiveDir, key)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		logs.Errorf("create archive dir %s failed, err: %v, rid: %s", dir, err, kt.Rid)
		return "", fmt.Errorf("create archive dir %s failed, err: %v", dir, err)
	}

	dest := filepath.Join(dir, version+".zip")

	req, err := http.NewRequestWithContext(kt.Ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		logs.Errorf("create download request failed, err: %v, rid: %s", err, kt.Rid)
		return "", fmt.Errorf("create download request failed, err: %v", err)
	}

	var httpClient = &http.Client{
		Timeout: time.Minute, // 设置60s超时
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		logs.Errorf("download zip http send failed, err: %v, rid: %s", err, kt.Rid)
		return "", fmt.Errorf("download zip http send failed, err: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download zip failed, err: unexpected status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		logs.Errorf("create zip file %s failed, err: %v, rid: %s", dest, err, kt.Rid)
		return "", fmt.Errorf("create zip file %s failed, err: %v", dest, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		if rErr := os.Remove(dest); rErr != nil {
			return "", fmt.Errorf("remove zip %s failed: %v", dest, rErr)
		}
		return "", fmt.Errorf("write zip %s failed: %v", dest, err)
	}
	return dest, nil
}

// extract unzips zipPath into skillRoot/{key}/ using an atomic temp→rename strategy.
func (inst *Installer) extract(kt *kit.Kit, zipPath, key string) error {
	targetDir := filepath.Join(inst.skillRoot, key)

	tmpDir, err := os.MkdirTemp(inst.skillRoot, ".skill-tmp-"+key+"-*")
	if err != nil {
		logs.Errorf("create temp dir for skill %s failed, err: %v, rid: %s", key, err, kt.Rid)
		return fmt.Errorf("create temp dir for skill %s: %w", key, err)
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		logs.Errorf("open zip %s failed, err: %v, rid: %s", zipPath, err, kt.Rid)
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if err = extractZipFile(kt, f, tmpDir); err != nil {
			logs.Errorf("extract file %s failed, err: %v, rid: %s", f.Name, err, kt.Rid)
			return fmt.Errorf("extract file %s: %w", f.Name, err)
		}
	}

	// 覆盖更新
	if err = os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("remove old skill dir %s: %w", targetDir, err)
	}
	if err = os.Rename(tmpDir, targetDir); err != nil {
		return fmt.Errorf("rename temp dir to %s: %w", targetDir, err)
	}
	return nil
}

// extractZipFile writes a single zip entry to destDir.
func extractZipFile(kt *kit.Kit, f *zip.File, destDir string) error {
	destPath := filepath.Join(destDir, filepath.FromSlash(f.Name))
	if len(destPath) < len(destDir) || destPath[:len(destDir)] != destDir {
		logs.Errorf("illegal path in zip: %s, rid: %s", f.Name, kt.Rid)
		return fmt.Errorf("illegal path in zip: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, defaultDirPerm)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), defaultDirPerm); err != nil {
		logs.Errorf("create destination dir %s failed, err: %v, rid: %s", destPath, err, kt.Rid)
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(out, rc)
	return err
}
