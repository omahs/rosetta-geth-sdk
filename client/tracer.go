// Copyright 2022 Coinbase, Inc.
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

package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

// convert raw eth data from SDKClient to rosetta

const (
	tracerPath = "client/call_tracer.js"
)

var (
	tracerTimeout = "120s"
	nativeTracer  = "callTracer"
)

func GetTraceConfig(useNative bool) (*tracers.TraceConfig, error) {
	if useNative {
		return &tracers.TraceConfig{
			Timeout: &tracerTimeout,
			Tracer:  &nativeTracer,
		}, nil
	}
	return loadTraceConfig()
}

func loadTraceConfig() (*tracers.TraceConfig, error) {
	loadedFile, err := ioutil.ReadFile(tracerPath)
	if err != nil {
		return nil, fmt.Errorf("%w: could not load tracer file", err)
	}

	loadedTracer := string(loadedFile)
	return &tracers.TraceConfig{
		Timeout: &tracerTimeout,
		Tracer:  &loadedTracer,
	}, nil
}

// geth traces types
type rpcCall struct {
	Result *Call `json:"result"`
}

// Call is an Ethereum debug trace.
type Call struct {
	Type         string         `json:"type"`
	From         common.Address `json:"from"`
	To           common.Address `json:"to"`
	Value        *big.Int       `json:"value"`
	GasUsed      *big.Int       `json:"gasUsed"`
	Revert       bool
	ErrorMessage string  `json:"error"`
	Calls        []*Call `json:"calls"`
}

type FlatCall struct {
	Type         string         `json:"type"`
	From         common.Address `json:"from"`
	To           common.Address `json:"to"`
	Value        *big.Int       `json:"value"`
	GasUsed      *big.Int       `json:"gasUsed"`
	Revert       bool
	ErrorMessage string `json:"error"`
}

func (t *Call) flatten() *FlatCall {
	return &FlatCall{
		Type:         t.Type,
		From:         t.From,
		To:           t.To,
		Value:        t.Value,
		GasUsed:      t.GasUsed,
		Revert:       t.Revert,
		ErrorMessage: t.ErrorMessage,
	}
}

// UnmarshalJSON is a custom unmarshaler for Call.
func (t *Call) UnmarshalJSON(input []byte) error {
	type CustomTrace struct {
		Type         string         `json:"type"`
		From         common.Address `json:"from"`
		To           common.Address `json:"to"`
		Value        *hexutil.Big   `json:"value"`
		GasUsed      *hexutil.Big   `json:"gasUsed"`
		Revert       bool
		ErrorMessage string  `json:"error"`
		Calls        []*Call `json:"calls"`
	}
	var dec CustomTrace
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	t.Type = dec.Type
	t.From = dec.From
	t.To = dec.To
	if dec.Value != nil {
		t.Value = (*big.Int)(dec.Value)
	} else {
		t.Value = new(big.Int)
	}
	if dec.GasUsed != nil {
		t.GasUsed = (*big.Int)(dec.GasUsed)
	} else {
		t.GasUsed = new(big.Int)
	}
	if dec.ErrorMessage != "" {
		// Any error surfaced by the decoder means that the Transaction
		// has reverted.
		t.Revert = true
	}
	t.ErrorMessage = dec.ErrorMessage
	t.Calls = dec.Calls
	return nil
}

// Open Ethereum API traces
type OpenEthTraceCall struct {
	Output string         `json:"output"`
	Trace  []OpenEthTrace `json:"trace"`
}

type OpenEthTrace struct {
	Subtraces       int64         `json:"subtraces"`
	Action          OpenEthAction `json:"action"`
	Type            string        `json:"type"`
	TransactionHash string        `json:"transactionHash"`
}

type OpenEthAction struct {
	Type    string         `json:"callType"`
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`
	Value   *big.Int       `json:"value"`
	GasUsed *big.Int       `json:"gas"`
}

// UnmarshalJSON is a custom unmarshaler for OpenEthAction.
func (t *OpenEthAction) UnmarshalJSON(input []byte) error {
	type CustomTrace struct {
		Type    string         `json:"callType"`
		From    common.Address `json:"from"`
		To      common.Address `json:"to"`
		Value   *hexutil.Big   `json:"value"`
		GasUsed *hexutil.Big   `json:"gas"`
	}
	var dec CustomTrace
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	t.Type = dec.Type
	t.From = dec.From
	t.To = dec.To
	if dec.Value != nil {
		t.Value = dec.Value.ToInt()
	} else {
		t.Value = new(big.Int)
	}
	if dec.GasUsed != nil {
		t.GasUsed = dec.GasUsed.ToInt()
	} else {
		t.GasUsed = new(big.Int)
	}
	return nil
}

// flattenTraces recursively flattens all traces.
func FlattenOpenEthTraces(data *OpenEthTraceCall, flattened []*FlatCall) []*FlatCall {
	for _, child := range data.Trace {
		action := child.Action
		traceType := action.Type
		if traceType == "" {
			traceType = child.Type
		}
		flattenCall := &FlatCall{
			Type:    traceType,
			From:    action.From,
			To:      action.To,
			Value:   action.Value,
			GasUsed: action.GasUsed,
			// Revert:       t.Revert,
			// ErrorMessage: t.ErrorMessage,
		}
		flattened = append(flattened, flattenCall)
	}
	return flattened
}
