// Copyright 2026 The Beijing Gu Liang Technology Co., Ltd. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cred

import (
	"crypto/md5" //nolint:gosec // G501: legacy hash verification for accounts created before argon2id
	"encoding/hex"
	"log/slog"
)

type Md5UserSaltCredManager struct{}

func getMd5(data []byte) []byte {
	hash := md5.Sum(data) //nolint:gosec // G401: legacy compatibility only; new passwords use argon2id/bcrypt
	return hash[:]
}

func getMd5HexDigest(s string) string {
	b := getMd5([]byte(s))
	res := hex.EncodeToString(b)
	return res
}

func NewMd5UserSaltCredManager() *Md5UserSaltCredManager {
	cm := &Md5UserSaltCredManager{}
	return cm
}

func (cm *Md5UserSaltCredManager) GetHashedPassword(password, salt string) string {
	// 新写入仍然可能走到这里（历史账号的改密流程），因此无论是否有盐都留痕，
	// 便于确认没有新账号继续以 MD5 落库。
	slog.Warn("MD5 password hashing is deprecated; use argon2id or bcrypt for new passwords")
	if salt == "" {
		return getMd5HexDigest(password)
	}

	return getMd5HexDigest(getMd5HexDigest(password) + salt)
}

func (cm *Md5UserSaltCredManager) IsPasswordCorrect(plainPwd, hashedPwd, salt string) bool {
	// For backward-compatibility
	if salt == "" {
		if hashedPwd == cm.GetHashedPassword(getMd5HexDigest(plainPwd), salt) {
			return true
		}
	}

	return hashedPwd == cm.GetHashedPassword(plainPwd, salt)
}
