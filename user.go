/*
NAME
  Perf360 is a simple web app for collecting 360-degree performance reviews.

AUTHOR
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (c) 2026, The Perf360 Authors

  BSD 3-Clause License

  Redistribution and use in source and binary forms, with or without
  modification, are permitted provided that the following conditions are met:

  1. Redistributions of source code must retain the above copyright notice, this
     list of conditions and the following disclaimer.

  2. Redistributions in binary form must reproduce the above copyright notice,
     this list of conditions and the following disclaimer in the documentation
     and/or other materials provided with the distribution.

  3. Neither the name of The Australian Ocean Lab Ltd. ("AusOcean")
     nor the names of its contributors may be used to endorse or promote
     products derived from this software without specific prior written permission.

  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
  AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
  DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
  FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
  DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
  SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
  CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
  OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
  OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ausocean/cloud/gauth"
)

// User represents a Perf360 user.
type User struct {
	Name      string
	Email     string
	Manager   string
	IsManager bool
	IsCEO     bool
}

// importUsers imports users in JSON format from a Google Storage
// Bucket or file specified by the PERF360_USERS environment
// variable. Reading from a bucket additionally requires secrets
// specified by PERF360_SECRETS.
func importUsers(ctx context.Context) ([]User, error) {
	url := os.Getenv("PERF360_USERS")
	if url == "" {
		return nil, errors.New("PERF360_USERS environment variable not defined")
	}

	var content []byte
	var err error
	if strings.HasPrefix(url, "gs://") {
		content, err = gauth.ReadGoogleStorageBucket(ctx, url)
	} else {
		content, err = ioutil.ReadFile(url)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read users from %s: %w", url, err)
	}

	var users []User
	err = json.Unmarshal([]byte(content), &users)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshall JSON from %s: %w", url, err)
	}
	return users, nil
}
