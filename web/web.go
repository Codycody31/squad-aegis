//go:build !embed
// +build !embed

// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"io"
	"net/http"
	"os"
)

func HTTPFS() (http.FileSystem, error) {
	// Use local filesystem directory
	return http.Dir(".output/public"), nil
}

func Lookup(path string) (buf []byte, err error) {
	f, err := os.Open(".output/public/" + path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
