package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Pilladian/go-helper"
	"github.com/Pilladian/logger"
)

// GLOBAL VARS
var TARGETS []string
var CLIENT *http.Client
var DOWN = make(map[string]int)
var INFOWATCH_PID = os.Getenv("INFOWATCH_PID")
var ELASTIC_SEARCH_PID = os.Getenv("ELASTIC_SEARCH_PID")

// Send Logs to InfoWatch
func sendLogsToInfoWatch(pid string, domain string, response_code string) (int, error) {
	request_body, request_body_err := json.Marshal(map[string]string{
		"domain":        domain,
		"response_code": response_code,
		"time":          time.Now().Format(time.RFC3339),
	})

	if request_body_err != nil {
		return 1, request_body_err
	}

	url := os.Getenv("INFOWATCH_URL")
	rev_proxy_username := os.Getenv("INFOWATCH_REV_PROXY_USERNAME")
	rev_proxy_password := os.Getenv("INFOWATCH_REV_PROXY_PASSWORD")

	req, _ := http.NewRequest("POST", fmt.Sprintf(url, pid), bytes.NewBuffer(request_body))
	req.SetBasicAuth(rev_proxy_username, rev_proxy_password)
	re, re_err := CLIENT.Do(req)
	if re_err != nil {
		return 1, re_err
	}
	return re.StatusCode, nil
}

func sendLogsToElastic(pid string, domain string, response_code int) (int, error) {
	request_body, request_body_err := json.Marshal(map[string]interface{}{
		"domain":        domain,
		"response_code": response_code,
		"time":          time.Now().Format(time.RFC3339),
	})

	if request_body_err != nil {
		logger.Error(request_body_err.Error())
		return 1, request_body_err
	}

	url := os.Getenv("ELASTIC_SEARCH_URL")
	rev_proxy_username := os.Getenv("ELASTIC_SEARCH_USERNAME")
	rev_proxy_password := os.Getenv("ELASTIC_SEARCH_PASSWORD")

	req, _ := http.NewRequest("POST", fmt.Sprintf(url+"/%s/_doc", pid), bytes.NewBuffer(request_body))
	req.SetBasicAuth(rev_proxy_username, rev_proxy_password)
	req.Header.Set("Content-Type", "application/json")
	re, re_err := CLIENT.Do(req)
	if re_err != nil {
		return 1, re_err
	}
	body, _ := io.ReadAll(re.Body)
	logger.Info("Elastic Response: " + string(body))
	return re.StatusCode, nil
}

// Return list of targets, that will be monitored
func getTargets() {
	for i := 1; i <= 20; i++ {
		e := os.Getenv(fmt.Sprintf("URL_%d", i))
		if e != "" {
			TARGETS = append(TARGETS, e)
			logger.Info(fmt.Sprintf("New Target found: %s", e))
		} else {
			return
		}
	}
}

// Check external proxy
//
//	51 : Request Error
//	52 : Read Body Error
func externalProxyCheck() (bool, int, error) {
	url := os.Getenv("EXTERNAL_PROXY_URL")
	req, _ := http.NewRequest("GET", url, nil)
	req.Host = os.Getenv("EXTERNAL_PROXY_HOST")
	re, re_err := CLIENT.Do(req)
	if re_err != nil {
		return false, 51, re_err
	}
	body, body_err := io.ReadAll(re.Body)
	if body_err != nil {
		return false, 52, body_err
	}
	defer re.Body.Close()

	if re.StatusCode == 200 && string(body) == "running" {
		return true, 200, nil
	} else {
		return false, re.StatusCode, nil
	}
}

// Check internal proxy
//
//	61 : Request Error
//	62 : Read Body Error
func internalProxyCheck() (bool, int, error) {
	url := os.Getenv("INTERNAL_PROXY_URL")
	req, _ := http.NewRequest("GET", url, nil)
	req.Host = os.Getenv("INTERNAL_PROXY_HOST")
	re, re_err := CLIENT.Do(req)
	if re_err != nil {
		return false, 61, re_err
	}
	body, body_err := io.ReadAll(re.Body)
	if body_err != nil {
		return false, 62, body_err
	}
	defer re.Body.Close()

	if re.StatusCode == 200 && string(body) == "running" {
		return true, 200, nil
	} else {
		return false, re.StatusCode, nil
	}
}

// Check URL
//
//	71 : Request Error
func urlCheck(url string) (bool, int, error) {
	req, _ := http.NewRequest("GET", url, nil)
	if strings.Contains(url, os.Getenv("ELASTIC_SEARCH_URL")) {
		req.SetBasicAuth(os.Getenv("ELASTIC_SEARCH_USERNAME"), os.Getenv("ELASTIC_SEARCH_PASSWORD"))
	}
	re, re_err := CLIENT.Do(req)
	if re_err != nil {
		return false, 71, re_err
	}

	if re.StatusCode == 200 {
		return true, 200, nil
	} else {
		return false, re.StatusCode, nil
	}
}

func main() {
	CHECK_INTERNAL_PROXY := false
	CHECK_EXTERNAL_PROXY := false

	logger.SetLogLevel(2)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	CLIENT = &http.Client{
		Timeout: 5 * time.Second,
	}

	getTargets()
	DOWN["ext_pr"] = -1
	DOWN["int_pr"] = -1
	for _, target := range TARGETS {
		sub := strings.Split(strings.Split(strings.Split(target, "https://")[1], "/")[0], ".")[0]
		DOWN[sub] = -1
	}

	m_ext_up := "\\[ u ] External"
	m_ext_down := "\\[ d ] External - %d"
	m_ext_still_down := "\\[ d ] External - %d"

	m_int_up := "\\[ u ] Internal"
	m_int_down := "\\[ d ] Internal - %d"
	m_int_still_down := "\\[ d ] Internal - %d"

	m_target_up := "\\[ u ] %s"
	m_target_down := "\\[ d ] %s - %d"
	m_target_still_down := "\\[ d ] %s - %d"

	for true {
		// EXTERNAL PROXY
		if CHECK_EXTERNAL_PROXY {
			logger.Info("Check External Proxy")
			ext_running, ext_response_code, _ := externalProxyCheck()
			// if external proxy was down the last scan
			if DOWN["ext_pr"] != -1 {
				// if external proxy is now up running
				if ext_running {
					DOWN["ext_pr"] = -1
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), m_ext_up)
					logger.Info(fmt.Sprintf(m_ext_up, ext_response_code))
					sendLogsToElastic(ELASTIC_SEARCH_PID, "external.proxy", ext_response_code)
				} else {
					// if external proxy is down for x minutes
					if DOWN["ext_pr"] == 360 {
						helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_ext_still_down, ext_response_code))
						DOWN["ext_pr"] = -1
						logger.Warning(fmt.Sprintf(m_ext_still_down, ext_response_code))
					}
					sendLogsToElastic(ELASTIC_SEARCH_PID, "external.proxy", ext_response_code)
					DOWN["ext_pr"]++
				}
			} else {
				// if external proxy was up last scan and is not running anymore
				if !ext_running {
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_ext_down, ext_response_code))
					DOWN["ext_pr"] = 0
					logger.Warning(fmt.Sprintf(m_ext_down, ext_response_code))
				}
				sendLogsToElastic(ELASTIC_SEARCH_PID, "external.proxy", ext_response_code)
			}
		}

		// INTERNAL PROXY
		if CHECK_INTERNAL_PROXY {
			logger.Info("Check Internal Proxy")
			int_running, int_response_code, _ := internalProxyCheck()
			// if internal proxy was down the last scan
			if DOWN["int_pr"] != -1 {
				// if internal proxy is now up running
				if int_running {
					DOWN["int_pr"] = -1
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), m_int_up)
					logger.Info(m_int_up)
					sendLogsToElastic(ELASTIC_SEARCH_PID, "internal.proxy", int_response_code)
				} else {
					// if internal proxy is down for x minutes
					if DOWN["int_pr"] == 360 {
						helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_int_still_down, int_response_code))
						DOWN["int_pr"] = -1
						logger.Warning(fmt.Sprintf(m_int_still_down, int_response_code))
					}
					sendLogsToElastic(ELASTIC_SEARCH_PID, "internal.proxy", int_response_code)
					DOWN["int_pr"]++
				}
			} else {
				// if internal proxy was up last scan and is not running anymore
				if !int_running {
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_int_down, int_response_code))
					DOWN["int_pr"] = 0
					logger.Warning(fmt.Sprintf(m_int_down, int_response_code))
				}
				sendLogsToElastic(ELASTIC_SEARCH_PID, "internal.proxy", int_response_code)
			}
		}

		// TARGETS
		for _, target := range TARGETS {
			domain := strings.Split(strings.Split(target, "https://")[1], "/")[0]
			sub := strings.Split(domain, ".")[0]
			logger.Info(fmt.Sprintf("Check %s", domain))
			target_running, target_response_code, _ := urlCheck(target)
			// if target was down the last scan
			if DOWN[sub] != -1 {
				// if target is now up running
				if target_running {
					DOWN[sub] = -1
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_target_up, sub))
					logger.Info(fmt.Sprintf(m_target_up, sub))
					sendLogsToElastic(ELASTIC_SEARCH_PID, domain, target_response_code)
				} else {
					// if target is down for x minutes
					if DOWN[sub] == 360 {
						helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_target_still_down, sub, target_response_code))
						DOWN[sub] = -1
						logger.Warning(fmt.Sprintf(m_target_still_down, sub, target_response_code))
					}
					sendLogsToElastic(ELASTIC_SEARCH_PID, domain, target_response_code)
					DOWN[sub]++
				}
			} else {
				// if target was up last scan and is not running anymore
				if !target_running {
					helper.SendTelegramMessage(os.Getenv("BOT_TOKEN"), os.Getenv("CHAT_ID"), fmt.Sprintf(m_target_down, sub, target_response_code))
					DOWN[sub] = 0
					logger.Warning(fmt.Sprintf(m_target_down, sub, target_response_code))
				}
				sendLogsToElastic(ELASTIC_SEARCH_PID, domain, target_response_code)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
