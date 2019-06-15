package tools

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
		Token      string `json:"token"`
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

// GetAnnouncement 获取公告
func GetAnnouncement() string {
	url := *toolsAPIAddr + "/api/v1/natproxy/annoncement"
	respJSON := &respJSON{}

	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return ""
	}

	if respJSON.Code != 200 {
		return ""
	}

	return respJSON.Msg
}

// Register 注册
func Register(email, password string) error {
	url := *toolsAPIAddr + "/api/v1/register"
	respJSON := &respJSON{}

	type Register struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	jsonBytes, err := json.Marshal(&Register{Email: email, Password: password})
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
		return fmt.Errorf("无法注册，原因是: %s", respJSON.Msg)
	}

	return nil
}

// Login 登录
func Login(email, password string) (string, error) {
	url := *toolsAPIAddr + "/api/v1/login"
	respJSON := &respJSON{}

	type Login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	jsonBytes, err := json.Marshal(&Login{Email: email, Password: password})
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return "", err
	}

	if respJSON.Code != 200 {
		return "", fmt.Errorf("无法登录，原因是: %s", respJSON.Msg)
	}

	return respJSON.Data.Token, nil
}

// Disconnect 是否断开连接
func Disconnect(token string, disconnect bool) error {
	url := *toolsAPIAddr + "/api/v1/natproxy/status"
	respJSON := &respJSON{}

	type Disconnect struct {
		Token      string `json:"token"`
		Disconnect bool   `json:"disconnect"`
	}
	jsonBytes, err := json.Marshal(&Disconnect{Token: token, Disconnect: disconnect})
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
		return fmt.Errorf("无法更新状态，原因是: %s", respJSON.Msg)
	}

	return nil
}
