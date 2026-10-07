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
	"time"

	"github.com/ausocean/cloud/datastore"
)

// typeReview is the name of the datastore type.
const typeReview = "Review"

// Review represents peer feedback given by one user (Name) to a peer (Peer).
// Name and Peer are user names. The unique key is the concatenation Period.Name.Peer.
type Review struct {
	Period   string
	Name     string
	Peer     string
	Feedback string `datastore:",noindex"`
	Locked   bool
	Updated  time.Time
}

// Encode serializes a Review into JSON.
func (r *Review) Encode() []byte {
	bytes, _ := json.Marshal(r)
	return bytes
}

// Decode deserializes a =Review from JSON.
func (r *Review) Decode(b []byte) error {
	return json.Unmarshal(b, r)
}

// Copy copies a Review to dst, or returns a copy of the site when dst is nil.
func (r *Review) Copy(dst datastore.Entity) (datastore.Entity, error) {
	var _r *Review
	if dst == nil {
		_r = new(Review)
	} else {
		var ok bool
		_r, ok = dst.(*Review)
		if !ok {
			return nil, datastore.ErrWrongType
		}
	}
	*_r = *r
	return _r, nil
}

// GetCache returns nil since we're not using caching.
func (r *Review) GetCache() datastore.Cache {
	return nil
}

// putReview creates or updates a review.
func putReview(ctx context.Context, store datastore.Store, r *Review) error {
	r.Period = reviewPeriod
	r.Updated = time.Now()
	key := store.NameKey(typeReview, reviewPeriod+"."+r.Name+"."+r.Peer)
	_, err := store.Put(ctx, key, r)
	return err
}

// getReview returns a single review.
func getReview(ctx context.Context, store datastore.Store, name, peer string) (*Review, error) {
	key := store.NameKey(typeReview, reviewPeriod+"."+name+"."+peer)
	var r Review
	err := store.Get(ctx, key, &r)
	if err != nil {
		return nil, err
	}

	return &r, err
}

// getReviews returns all of the reviews for the current review period.
func getReviews(ctx context.Context, store datastore.Store) ([]Review, error) {
	q := store.NewQuery(typeReview, false, "Period")
	q.Filter("Period =", reviewPeriod)
	var reviews []Review
	_, err := store.GetAll(ctx, q, &reviews)
	return reviews, err
}

// getPeerReviews returns all of the reviews for a peer for the current review period.
func getPeerReviews(ctx context.Context, store datastore.Store, peer string) ([]Review, error) {
	q := store.NewQuery(typeReview, false, "Period", "Peer")
	q.Filter("Period =", reviewPeriod)
	q.Filter("Peer =", peer)
	var reviews []Review
	_, err := store.GetAll(ctx, q, &reviews)
	return reviews, err
}
