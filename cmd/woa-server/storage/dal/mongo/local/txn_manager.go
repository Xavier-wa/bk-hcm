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
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"hcm/cmd/woa-server/storage/dal/mongo/uuid"
	"hcm/pkg"
	"hcm/pkg/tools/metadata"

	"go.mongodb.org/mongo-driver/mongo"
)

// TxnManager TODO
// a transaction manager
type TxnManager struct{}

// ReloadSession is used to reset a created session's session id
func (t *TxnManager) ReloadSession(sess mongo.Session, info *SessionInfo) (mongo.Session, error) {
	err := CmdbReloadSession(sess, info)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// PrepareCommit prepare transaction commit
func (t *TxnManager) PrepareCommit(cli *mongo.Client) (mongo.Session, error) {
	// create a session client.
	sess, err := cli.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start session failed, err: %v", err)
	}
	return sess, nil
}

// PrepareTransaction prepare transaction
func (t *TxnManager) PrepareTransaction(cap *metadata.TxnCapable, cli *mongo.Client) (mongo.Session, error) {
	// create a session client.
	sess, err := cli.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start session failed, err: %v", err)
	}

	// only for changing the transaction status
	err = sess.StartTransaction()
	if err != nil {
		return nil, fmt.Errorf("start transaction %s failed: %v", cap.SessionID, err)
	}

	// reset the session info with the session id.
	info := &SessionInfo{
		SessionID: cap.SessionID,
	}

	err = CmdbReloadSession(sess, info)
	if err != nil {
		return nil, fmt.Errorf("reload transaction: %s failed, err: %v", cap.SessionID, err)
	}

	return sess, nil
}

// GetTxnContext create a session if the ctx is a session context, and the bool value is true.
// so the caller must check the bool, and use session only when the bool is true.
// otherwise the caller should not use the session, should call the mongodb command directly.
// Note: this function is always used with mongo.CmdbReleaseSession(ctx, sessCtx) to release the session connection.
func (t *TxnManager) GetTxnContext(ctx context.Context, cli *mongo.Client) (context.Context, mongo.Session, bool,
	error) {

	cap, useTxn, err := parseTxnInfoFromCtx(ctx)
	if err != nil {
		return ctx, nil, false, err
	}

	if !useTxn {
		// not use transaction, return directly.
		return ctx, nil, false, nil
	}

	session, err := t.PrepareTransaction(cap, cli)
	if err != nil {
		return ctx, nil, true, err
	}

	// prepare the session context, it tells the driver to run this within a transaction.
	sessCtx := CmdbContextWithSession(ctx, session)

	return sessCtx, session, true, nil
}

// parseTxnInfoFromCtx try to parse transaction info from context,
// it returns the TxnCable, and a bool to indicate whether it's a transaction context or not.
// so the caller can use the returned TxnCapable only when the bool is true. otherwise it will be panic.
func parseTxnInfoFromCtx(txnCtx context.Context) (*metadata.TxnCapable, bool, error) {
	id := txnCtx.Value(pkg.TransactionIdHeader)
	if id == nil {
		// do not use transaction, and return directly.
		return nil, false, nil
	}

	txnID, ok := id.(string)
	if !ok {
		return nil, false, fmt.Errorf("invalid transaction id value： %v", id)
	}

	// parse timeout
	ttl := txnCtx.Value(pkg.TransactionTimeoutHeader)
	if ttl == nil {
		return nil, false, errors.New("transaction timeout value not exist")
	}

	ttlStr, ok := ttl.(string)
	if !ok {
		return nil, false, fmt.Errorf("invalid transaction timeout value: %v", ttl)
	}

	timeout, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil {
		return nil, false, fmt.Errorf("invalid transaction timeout value, parse %v failed, err: %v", ttl, err)
	}

	cap := &metadata.TxnCapable{
		// timeout is not
		Timeout:   time.Duration(timeout),
		SessionID: txnID,
	}
	return cap, true, nil
}

// AutoRunWithTxn auto run with transaction
func (t *TxnManager) AutoRunWithTxn(ctx context.Context, cli *mongo.Client, cmd func(ctx context.Context) error) error {
	cap, useTxn, err := parseTxnInfoFromCtx(ctx)
	if err != nil {
		return err
	}

	if !useTxn {
		// not use transaction, run command directly.
		return cmd(ctx)
	}

	session, err := t.PrepareTransaction(cap, cli)
	if err != nil {
		return err
	}

	// prepare the session context, it tells the driver to run this within a transaction.
	sessCtx := CmdbContextWithSession(ctx, session)

	// run the command and check error
	err = cmd(sessCtx)
	if err != nil {
		return err
	}
	// release the session connection.
	// Attention: do not use session.EndSession() to do this, it will abort the transaction.
	// mongo.CmdbReleaseSession(ctx, session)
	return nil
}

// GenSessionID TODO
func GenSessionID() (string, error) {
	// mongodb driver used this as it's mongodb session id, and we use it too.
	id, err := uuid.New()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(id[:]), nil
}

// GenTxnCableAndSetHeader TODO
// generate a session id and set it to header.
func GenTxnCableAndSetHeader(header http.Header, opts ...metadata.TxnOption) (*metadata.TxnCapable, error) {
	sessionID, err := GenSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id failed, err: %v", err)
	}
	var timeout time.Duration
	if len(opts) != 0 {
		if opts[0].Timeout < 30*time.Second {
			timeout = pkg.TransactionDefaultTimeout
		} else {
			timeout = opts[0].Timeout
		}
	} else {
		// set default value
		timeout = pkg.TransactionDefaultTimeout
	}

	header.Set(pkg.TransactionIdHeader, sessionID)
	header.Set(pkg.TransactionTimeoutHeader, strconv.FormatInt(int64(timeout), 10))

	cap := metadata.TxnCapable{
		Timeout:   timeout,
		SessionID: sessionID,
	}
	return &cap, nil
}
