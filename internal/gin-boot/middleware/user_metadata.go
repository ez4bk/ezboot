/*
 * Copyright (C) 2019 LambdaCal Inc.
 */

package middleware

import (
	"context"
	"strconv"

	"github.com/ez4bk/ezboot/internal/gin-boot/util"
)

// UserKey userKey
const UserKey = "user_id"

// UserNameKey UserNameKey
const UserNameKey = "user_name"

// GetUserID returns the user id from the incoming gRPC metadata.
// ISSUE: The int64 id is only used in onpremise, platform is using snowflake ID.
// Future refactoring is needed.
func GetUserID(ctx context.Context) (int64, error) {
	user, err := GetUserIDString(ctx)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(user, 10, 64)
}

// GetUserIDString returns the raw user id from the incoming metadata.
func GetUserIDString(ctx context.Context) (string, error) {
	return util.GetInMetadata(ctx, UserKey)
}

// WithUserID stores the user id in the incoming metadata.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return util.SetInMetadata(ctx, UserKey, strconv.FormatInt(userID, 10))
}

// GetUserName returns the user name from the incoming metadata.
func GetUserName(ctx context.Context) (string, error) {
	return util.GetInMetadata(ctx, UserNameKey)
}
