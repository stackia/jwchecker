package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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

func (self *StudentInfo) ScanFromInput() {
OUTER:
	for {
		fmt.Print("请输入你的学号: ")
		fmt.Scan(&self.Number)
		for _, v := range self.Number {
			if !unicode.IsDigit(v) {
				fmt.Println("学号无效，请重新输入。")
				continue OUTER
			}
		}

		fmt.Print("请输入你的教务系统登录密码: ")
		fmt.Scan(&self.Password)
		break
	}

}

func main() {
	var student StudentInfo

	// Input number and password
	student.ScanFromInput()

	// Input the subject name to check
	var subjectName string
	fmt.Print("请输入要查询的课程名称: ")
	fmt.Scan(&subjectName)

	log.Println("开始尝试...")

	var lastUrl *url.URL
	var lastBodyContent string
	var client *http.Client

	// Handle panic
	defer func() {
		if err := recover(); err != nil {
			log.Println("发生异常，错误如下:\n", err)
			fmt.Println("输出调试信息？(y/n)")
			var choice string
			if fmt.Scan(&choice); choice == "y" {
				fmt.Println(lastBodyContent)
			}
			os.Exit(1)
		}
	}()

	client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			lastUrl = req.URL
			return nil
		},
	}

	// Get login entrance
	resp, err := client.Get(baseUrl)
	defer resp.Body.Close()
	if err != nil {
		panic(err)
	}
	loginUrl := lastUrl
	log.Println("取得教务登录入口:", loginUrl)

	// Get login hash
	reg := regexp.MustCompile("\\/%28(.*)%29\\/")
	match := reg.FindAllStringSubmatch(loginUrl.String(), 1)
	loginHash := match[0][1]
	log.Println("Hash:", loginHash)

	// Get __VIEWSTATE
	body, _ := ioutil.ReadAll(resp.Body)
	lastBodyContent = string(body)
	viewState := getViewState(&lastBodyContent)

	for {
		// Do login
		loginFormValues := url.Values{"__VIEWSTATE": {viewState}, "TextBox1": {student.Number}, "TextBox2": {student.Password}, "TextBox3": {""}, "Button1": {""}, "lbLanguage": {""}, "RadioButtonList1": {"学生"}}
		resp, err = client.PostForm(loginUrl.String(), loginFormValues)
		defer resp.Body.Close()
		if err != nil {
			panic(err)
		}

		// Get student number and name
		cd, _ := iconv.Open("utf-8", "gbk")
		body, _ = ioutil.ReadAll(resp.Body)
		lastBodyContent = cd.ConvString(string(body))
		reg = regexp.MustCompile("\\<span id=\"xhxm\"\\>(\\d*)  (.*)同学\\<\\/span\\>")
		match = reg.FindAllStringSubmatch(lastBodyContent, 1)
		if len(match) == 0 {
			log.Println("登录失败，请重新输入学号密码。")
			student.ScanFromInput()
			continue
		}
		log.Println("已登录，姓名:", match[0][2])
		break
	}

	// Get grade query URL
	reg = regexp.MustCompile("href=\"(xscjcx.aspx\\?xh=\\d*&xm=.*&gnmkdm=.*)\" target.*成绩查询")
	match = reg.FindAllStringSubmatch(lastBodyContent, 1)
	gradeQueryUrlStr := match[0][1]
	log.Println("取得成绩查询入口:", gradeQueryUrlStr)

	// Load grade page to get __VIEWSTATE
	gradeQueryUrl, _ := url.Parse(baseUrl + "(" + loginHash + ")/" + gradeQueryUrlStr)
	refererUrl, _ := url.Parse(baseUrl + "(" + loginHash + ")/xs_main.aspx?xh=" + student.Number)
	req, _ := http.NewRequest("GET", gradeQueryUrl.String(), bytes.NewBufferString(""))
	req.Header.Add("Referer", refererUrl.String())
	resp, err = client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		panic(err)
	}

	// Get __VIEWSTATE
	body, _ = ioutil.ReadAll(resp.Body)
	lastBodyContent = string(body)
	viewState = getViewState(&lastBodyContent)

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
		panic(err)
	}

	cd, _ := iconv.Open("utf-8", "gbk")
	body, _ = ioutil.ReadAll(resp.Body)
	lastBodyContent = cd.ConvString(string(body))

	count := strings.Count(lastBodyContent, subjectName)
	if count > 0 {
		log.Println(subjectName + "成绩已出！")
	} else {
		log.Println(subjectName + "成绩还没出。")
	}
}
