/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"crypto/rand"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateName(t *testing.T) {
	if err := ValidateName("localhost"); err != nil {
		t.Errorf("error %v", err)
	}
	if err := ValidateName("127.0.0.1"); err != nil {
		t.Errorf("error %v", err)
	}
	if err := ValidateName("local:host"); err == nil {
		t.Errorf("error")
	}
}

func uuid() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}

	uuid[8] = uuid[8]&^0xc0 | 0x80
	uuid[6] = uuid[6]&^0xf0 | 0x40

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

func TestDirFunc(t *testing.T) {
	uuid, err := uuid()
	if err != nil {
		t.Errorf("%v", err)
	}

	dir := "/tmp/egress/" + uuid
	if err := MkdirAll(dir); err != nil {
		t.Errorf("%v", err)
	}
	if !IsDirEmpty(dir) {
		t.Errorf("directory is not empty")
	}
	if err := BackupAndCleanDir(dir); err != nil {
		t.Errorf("%v", err)
	}
	if err := RemoveAll(dir); err != nil {
		t.Errorf("%v", err)
	}
	if err := RemoveAll(dir + "_bak"); err != nil {
		t.Errorf("%v", err)
	}
}

func TestIsDirEmpty(t *testing.T) {
	name, _ := uuid()
	if flag := IsDirEmpty(name); !flag {
		t.Errorf("not exist dir is empty")
	}
}

func TestExpandDir(t *testing.T) {
	if !filepath.IsAbs(ExpandDir("testExpandDir")) {
		t.Errorf("should return abs path")
	}
}

func TestBackupAndCleanDir(t *testing.T) {
	if BackupAndCleanDir("notexistdir") != nil {
		t.Errorf("nil for not exist dir")
	}
}

func TestNormalizeZapLogPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		for i := 0; i < 10; i++ {
			name, _ := uuid()
			if NormalizeZapLogPath(name) != name {
				t.Errorf("for non-windows should return same result")
			}
		}
	}
}
