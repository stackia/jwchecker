package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/qiniu/iconv"
)

const baseUrl string = "http://222.201.132.113/"

type StudentInfo struct {
	Number   string
	Password string
	Name     string
}

func getViewState(s *string) string {
	reg := regexp.MustCompile("name=\"__VIEWSTATE\" value=\"(.*)\"")
	match := reg.FindAllStringSubmatch(*s, 1)
	return match[0][1]
}

func (self *StudentInfo) ScanFromInput() error {
	fmt.Print("请输入你的学号: ")
	fmt.Scan(&self.Number)
	for _, v := range self.Number {
		if !unicode.IsDigit(v) {
			return errors.New("学号无效，请重新输入。")
		}
	}

	fmt.Print("请输入你的教务系统登录密码: ")
	fmt.Scan(&self.Password)

	return nil
}

func main() {
	var student StudentInfo

	// Input number and password
	for {
		err := student.ScanFromInput()
		if err == nil {
			break
		}
		fmt.Println(err)
	}

	// Input the subject name to check
	var subjectName string
	fmt.Print("请输入要查询的课程名称: ")
	fmt.Scan(&subjectName)

	// Handle panic
	defer func() {
		if err := recover(); err != nil {
			log.Fatal("发生异常，错误如下:\n", err)
		}
	}()

	var lastUrl *url.URL
	var client *http.Client

	redirectCheck := func(req *http.Request, via []*http.Request) error {
		lastUrl = req.URL
		return nil
	}

	client = &http.Client{
		CheckRedirect: redirectCheck,
	}

	resp, err := client.Get(baseUrl)
	defer resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	loginUrl := lastUrl
	client.CheckRedirect = nil
	fmt.Println("取得教务登录 URL:", loginUrl)

	reg := regexp.MustCompile("\\/%28(.*)%29\\/")
	match := reg.FindAllStringSubmatch(loginUrl.String(), 1)
	loginHash := match[0][1]
	fmt.Println("登录状态 Hash:", loginHash)

	// Get __VIEWSTATE
	body, _ := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)
	viewState := getViewState(&bodyStr)
	loginFormValues := url.Values{"__VIEWSTATE": {viewState}, "TextBox1": {student.Number}, "TextBox2": {student.Password}, "TextBox3": {""}, "Button1": {""}, "lbLanguage": {""}, "RadioButtonList1": {"学生"}}

	// Do login
	resp, err = client.PostForm(loginUrl.String(), loginFormValues)
	if err != nil {
		log.Fatal(err)
	}

	// Get student number and name
	cd, _ := iconv.Open("utf-8", "gbk")
	body, _ = ioutil.ReadAll(resp.Body)
	bodyStr = cd.ConvString(string(body))
	reg = regexp.MustCompile("\\<span id=\"xhxm\"\\>(\\d*)  (.*)同学\\<\\/span\\>")
	match = reg.FindAllStringSubmatch(bodyStr, 1)
	fmt.Println("已登录，姓名:", match[0][2])

	// Get grade query URL
	reg = regexp.MustCompile("href=\"(xscjcx.aspx\\?xh=\\d*&xm=.*&gnmkdm=.*)\" target.*成绩查询")
	match = reg.FindAllStringSubmatch(bodyStr, 1)
	gradeQueryUrlStr := match[0][1]
	fmt.Println("成绩查询入口:", gradeQueryUrlStr)

	// Load grade page to get __VIEWSTATE
	gradeQueryUrl, _ := url.Parse(baseUrl + "(" + loginHash + ")/" + gradeQueryUrlStr)
	refererUrl, _ := url.Parse(baseUrl + "(" + loginHash + ")/xs_main.aspx?xh=" + student.Number)
	req, _ := http.NewRequest("GET", gradeQueryUrl.String(), bytes.NewBufferString(""))
	req.Header.Add("Referer", refererUrl.String())
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	// Get __VIEWSTATE
	body, _ = ioutil.ReadAll(resp.Body)
	bodyStr = string(body)
	viewState = getViewState(&bodyStr)

	queryFormValues := url.Values{
		"__EVENTTARGET":   {""},
		"__EVENTARGUMENT": {""},
		"__VIEWSTATE":     {viewState},
		"hidLanguage":     {""},
		"ddlXN":           {""},
		"ddlXQ":           {""},
		"ddl_kcxz":        {""},
		"btn_zcj":         {"历年成绩"},
	}

	// Query grade
	req, _ = http.NewRequest("POST", gradeQueryUrl.String(), bytes.NewBufferString(queryFormValues.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(queryFormValues.Encode())))
	req.Header.Add("Referer", gradeQueryUrl.String())
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	cd, _ = iconv.Open("utf-8", "gbk")
	body, _ = ioutil.ReadAll(resp.Body)
	gradeResult := cd.ConvString(string(body))

	count := strings.Count(gradeResult, subjectName)
	if count > 0 {
		fmt.Println(subjectName + "成绩已出！")
	} else {
		fmt.Println(subjectName + "成绩还没出。")
	}
}
