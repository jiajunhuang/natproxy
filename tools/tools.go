package tools

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"

	"github.com/jiajunhuang/natproxy/errors"
)

var (
	toolsAPIAddr = flag.String("toolsAPI", "https://tools.jiajunhuang.com", "tools API")
)

type respJSON struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Disconnect bool   `json:"disconnect"`
		Addr       string `json:"addr"`
	}
}

// GetConnectionStatusByToken 根据token拿连接信息
func GetConnectionStatusByToken(token string) (bool, error) {
	url := *toolsAPIAddr + "/api/v1/natproxy/check_token?token=" + token
	respJSON := &respJSON{}

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return false, err
	}

	if respJSON.Code == 200 {
		return respJSON.Data.Disconnect, nil
	}

	return false, errors.ErrTokenNotValid
}

// GetAddrByToken 根据token拿已分配的公网地址
func GetAddrByToken(token string) (string, error) {
	url := *toolsAPIAddr + "/api/v1/natproxy/check_token?token=" + token
	respJSON := &respJSON{}

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return "", err
	}

	if respJSON.Code == 200 {
		return respJSON.Data.Addr, nil
	}

	return "", errors.ErrTokenNotValid
}

// CheckIfAddrAlreadyTaken 检查地址是否已经被分配
func CheckIfAddrAlreadyTaken(addr string) (bool, error) {
	url := *toolsAPIAddr + "/api/v1/natproxy/addr?addr=" + addr
	respJSON := &respJSON{}

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return false, err
	}

	if respJSON.Code == 200 {
		return true, nil
	}

	return false, nil
}

// RegisterAddr 注册地址
func RegisterAddr(token, addr string) error {
	// 向中心注册这个地址
	url := *toolsAPIAddr + "/api/v1/natproxy/addr"
	respJSON := &respJSON{}

	type RegisterAddr struct {
		Token string `json:"token"`
		Addr  string `json:"addr"`
	}
	jsonBytes, err := json.Marshal(&RegisterAddr{Token: token, Addr: addr})
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return err
	}

	if respJSON.Code != 200 {
		return errors.ErrFailedToRegisterAddr
	}

	return nil
}
