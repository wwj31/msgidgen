package main

import (
	"bufio"
	"flag"
	"fmt"
	"hash/fnv"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
)

const (
	msgIDTpl = `//Code generated by msgidgen. DO NOT EDIT.
syntax = "proto3";
package {{.PackName}};
option  go_package = "/{{.PackName}}";

enum Msg {
    {{.Content}}}
`

	msgIDTagTpl = `//Code generated by msgidgen. DO NOT EDIT.

package {{.PackName}}

var MsgIDTags = map[int32]string{
{{.Tags}}}
`
)

var (
	protoPath   = flag.String("path", "./", "the proto file path")
	tagPath     = flag.String("tag", "./", "the msg tag file path")
	packageName = flag.String("pack", "", "the package name")
	upper       = flag.Bool("upper", false, "upper all word")
	prefix      = flag.String("prefix", "", "the package name")
)

type (
	MsgID struct {
		PackName string
		Content  string
	}
	MsgTag struct {
		PackName string
		Tags     string
	}
)

const (
	msgIDFile  = "msgid.proto"
	msgTagFile = "msgtag.go"
)

func main() {
	flag.Parse()

	if *packageName == "" {
		log.Fatalf("package name is essential,-pack=")
	}

	tmpl, err := template.New("protoMsgIDTpl").Parse(msgIDTpl)
	if err != nil {
		log.Fatalf("error parse template:%v", err)
	}

	req, _ := glob.Compile("*Req")
	rsp, _ := glob.Compile("*Rsp")
	ntf, _ := glob.Compile("*Ntf")
	tag, _ := glob.Compile("tag:*")

	var (
		content string
		tagMap  string
	)
	err = filepath.WalkDir(*protoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatalf("error encountered:%v", err)
		}

		if d.Name() == msgIDFile {
			return nil
		}

		if filepath.Ext(d.Name()) != ".proto" {
			return nil
		}

		file, openErr := os.Open(path)
		if openErr != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			strData := strings.Split(scanner.Text(), " ")
			if len(strData) < 2 {
				continue
			}

			head := strData[0]
			body := strData[1]

			tagStr := ""
			for _, str := range strData {
				if tag.Match(str) {
					tagStr = str
					break
				}
			}

			if head == "message" && (req.Match(body) || rsp.Match(body) || ntf.Match(body)) {
				body = *prefix + extractWordsAndToUpper(body)
				numStr := hashStringToInt64(body)
				content += fmt.Sprintf("    %v = %v;", body, numStr)
				if tagStr != "" {
					val := strings.Split(tagStr, ":")[1]
					content += "// dispatch to " + val
					tagMap += fmt.Sprintf("    %v : \"%v\",\n", numStr, val)
				}
				content += "\n"
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("walk dir error:%v", err)
	}

	firstMsgID := extractWordsAndToUpper("    "+*prefix+"Unknown") + " = 0;\n"
	msgFile, _ := os.Create(filepath.Join(*protoPath, msgIDFile))
	err = tmpl.Execute(msgFile, &MsgID{
		PackName: *packageName,
		Content:  firstMsgID + content,
	})

	if tagMap != "" {
		tagTmpl, err := template.New("msgTagTpl").Parse(msgIDTagTpl)
		if err != nil {
			log.Fatalf("error parse template:%v", err)
		}

		tagFile, _ := os.Create(filepath.Join(*tagPath, msgTagFile))
		err = tagTmpl.Execute(tagFile, &MsgTag{
			PackName: *packageName,
			Tags:     tagMap,
		})

		tagFile.Sync()
		tagFile.Close()
	}

	msgFile.Sync()
	msgFile.Close()
}

func extractWordsAndToUpper(input string) string {
	// 使用正则表达式提取单词
	regex := regexp.MustCompile(`\b\w+\b`)
	words := regex.FindAllString(input, -1)

	var result string
	// 将每个单词转换为大写

	for i, word := range words {
		if *upper {
			words[i] = strings.ToUpper(word)
		}
		result += words[i]
	}

	return result
}

func hashStringToInt64(str string) int64 {
	h := fnv.New32()
	h.Write([]byte(str))
	hashValue := h.Sum32()
	return int64(hashValue%math.MaxInt32 - 1)
}
