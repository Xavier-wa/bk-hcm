/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package local

import (
	"context"
	"fmt"
	"strings"

	"hcm/pkg"
	"hcm/pkg/logs"
	"hcm/pkg/tools/metadata"
)

// CommitTransaction 提交事务
func (c *Mongo) CommitTransaction(ctx context.Context, cap *metadata.TxnCapable) error {
	rid := ctx.Value(pkg.ContextRequestIDField)

	reloadSession, err := c.tm.PrepareTransaction(cap, c.dbc)
	if err != nil {
		logs.Errorf("commit transaction, but prepare transaction failed, err: %v, rid: %v", err, rid)
		return err
	}
	// reset the transaction state, so that we can commit the transaction after start the
	// transaction immediately.
	if err := CmdbPrepareCommitOrAbort(reloadSession); err != nil {
		logs.Errorf("reset the commit transaction state failed, err: %v, rid: %v", err, rid)
		return err
	}

	// we commit the transaction with the session id
	err = reloadSession.CommitTransaction(ctx)
	if err != nil {
		// no operation executed in transaction, commit may return NoSuchTransaction, skip this case.
		if strings.Contains(err.Error(), "NoSuchTransaction") {
			logs.Infof("commit transaction: %s but no transaction need to commit, *skip*, rid: %s", cap.SessionID, rid)
			return nil
		}
		return fmt.Errorf("commit transaction: %s failed, err: %v, rid: %v", cap.SessionID, err, rid)
	}

	return nil
}

// AbortTransaction 取消事务
func (c *Mongo) AbortTransaction(ctx context.Context, cap *metadata.TxnCapable) (bool, error) {
	rid := ctx.Value(pkg.ContextRequestIDField)
	reloadSession, err := c.tm.PrepareTransaction(cap, c.dbc)
	if err != nil {
		logs.Errorf("abort transaction, but prepare transaction failed, err: %v, rid: %v", err, rid)
		return false, err
	}
	// reset the transaction state, so that we can abort the transaction after start the
	// transaction immediately.
	if err := CmdbPrepareCommitOrAbort(reloadSession); err != nil {
		logs.Errorf("reset abort transaction state failed, err: %v, rid: %v", err, rid)
		return false, err
	}

	// we abort the transaction with the session id
	err = reloadSession.AbortTransaction(ctx)
	if err != nil {
		// no operation executed in transaction, abort may return NoSuchTransaction, skip this case.
		if strings.Contains(err.Error(), "NoSuchTransaction") {
			logs.Infof("abort transaction: %s but no transaction need to abort, *skip*, rid: %s", cap.SessionID, rid)
			return false, nil
		}
		return false, fmt.Errorf("abort transaction: %s failed, err: %v, rid: %v", cap.SessionID, err, rid)
	}

	return false, nil
}
