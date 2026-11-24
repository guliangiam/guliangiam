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

import "github.com/alexedwards/argon2id"

type Argon2idCredManager struct{}

func NewArgon2idCredManager() *Argon2idCredManager {
	cm := &Argon2idCredManager{}
	return cm
}

func (cm *Argon2idCredManager) GetHashedPassword(password, salt string) string {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return ""
	}
	return hash
}

func (cm *Argon2idCredManager) IsPasswordCorrect(plainPwd, hashedPwd, salt string) bool {
	match, _ := argon2id.ComparePasswordAndHash(plainPwd, hashedPwd)
	return match
}

// HashArgon2id generates an argon2id hash from the given password.
func HashArgon2id(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// VerifyArgon2id verifies a password against an argon2id hash.
func VerifyArgon2id(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}
