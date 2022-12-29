// Copyright 2022-present Kuei-chun Chen. All rights reserved.

package hatchet

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/simagix/gox"
)

var ops = []string{cmdAggregate, cmdCount, cmdDelete, cmdDistinct, cmdFind,
	cmdFindAndModify, cmdGetMore, cmdInsert, cmdUpdate}

const cmdAggregate = "aggregate"
const cmdCount = "count"
const cmdCreateIndexes = "createIndexes"
const cmdDelete = "delete"
const cmdDistinct = "distinct"
const cmdFind = "find"
const cmdFindAndModify = "findandmodify"
const cmdGetMore = "getMore"
const cmdInsert = "insert"
const cmdRemove = "remove"
const cmdUpdate = "update"

// AnalyzeSlowOp analyzes slow ops
func AnalyzeSlowOp(doc *Logv2Info) (OpStat, error) {
	var err error
	var stat = OpStat{}

	c := doc.Component
	if c != "COMMAND" && c != "QUERY" && c != "WRITE" {
		return stat, errors.New("unsupported command")
	}
	stat.TotalMilli = doc.Attributes.Milli
	stat.Namespace = doc.Attributes.NS
	if stat.Namespace == "" {
		return stat, errors.New("no namespace found")
	} else if strings.HasPrefix(stat.Namespace, "admin.") || strings.HasPrefix(stat.Namespace, "config.") || strings.HasPrefix(stat.Namespace, "local.") {
		stat.Op = DOLLAR_CMD
		return stat, errors.New("system database")
	} else if strings.HasSuffix(stat.Namespace, ".$cmd") {
		stat.Op = DOLLAR_CMD
		return stat, errors.New("system command")
	}

	if doc.Attributes.PlanSummary != "" { // not insert
		plan := doc.Attributes.PlanSummary
		if strings.HasPrefix(plan, "IXSCAN") {
			stat.Index = plan[len("IXSCAN")+1:]
		} else {
			stat.Index = plan
		}
	}
	stat.Reslen = doc.Attributes.Reslen
	if doc.Attributes.Command == nil {
		return stat, errors.New("no command found")
	}
	command := doc.Attributes.Command
	stat.Op = doc.Attributes.Type
	if stat.Op == "command" || stat.Op == "none" {
		stat.Op = getOp(command)
	}
	var isGetMore bool
	if stat.Op == cmdGetMore {
		isGetMore = true
		command = doc.Attributes.OriginatingCommand
		stat.Op = getOp(command)
	}
	if stat.Op == cmdInsert || stat.Op == cmdCreateIndexes {
		stat.QueryPattern = "N/A"
	} else if (stat.Op == cmdUpdate || stat.Op == cmdRemove || stat.Op == cmdDelete) && stat.QueryPattern == "" {
		var query interface{}
		if command["q"] != nil {
			query = command["q"]
		} else if command["query"] != nil {
			query = command["query"]
		}

		if query != nil {
			walker := gox.NewMapWalker(cb)
			doc := walker.Walk(query.(map[string]interface{}))
			if buf, err := json.Marshal(doc); err == nil {
				stat.QueryPattern = string(buf)
			} else {
				stat.QueryPattern = "{}"
			}
		} else {
			return stat, errors.New("no filter found")
		}
	} else if stat.Op == cmdAggregate {
		pipeline, ok := command["pipeline"].([]interface{})
		if !ok || len(pipeline) == 0 {
			return stat, errors.New("pipeline not found")
		}
		var stage interface{}
		for _, v := range pipeline {
			stage = v
			break
		}
		fmap := stage.(map[string]interface{})
		if !isRegex(fmap) {
			walker := gox.NewMapWalker(cb)
			doc := walker.Walk(fmap)
			if buf, err := json.Marshal(doc); err == nil {
				stat.QueryPattern = string(buf)
			} else {
				stat.QueryPattern = "{}"
			}
			if !strings.Contains(stat.QueryPattern, "$match") && !strings.Contains(stat.QueryPattern, "$sort") &&
				!strings.Contains(stat.QueryPattern, "$facet") && !strings.Contains(stat.QueryPattern, "$indexStats") {
				stat.QueryPattern = "{}"
			}
		} else {
			buf, _ := json.Marshal(fmap)
			str := string(buf)
			re := regexp.MustCompile(`{(.*):{"\$regularExpression":{"options":"(\S+)?","pattern":"(\^)?(\S+)"}}}`)
			stat.QueryPattern = re.ReplaceAllString(str, "{$1:/$3.../$2}")
		}
	} else {
		var fmap map[string]interface{}
		if command["filter"] != nil {
			fmap = command["filter"].(map[string]interface{})
		} else if command["query"] != nil {
			fmap = command["query"].(map[string]interface{})
		} else if command["q"] != nil {
			fmap = command["q"].(map[string]interface{})
		} else {
			return stat, errors.New("no filter found")
		}
		if !isRegex(fmap) {
			walker := gox.NewMapWalker(cb)
			doc := walker.Walk(fmap)
			var data []byte
			if data, err = json.Marshal(doc); err != nil {
				return stat, err
			}
			stat.QueryPattern = string(data)
			if stat.QueryPattern == `{"":null}` {
				stat.QueryPattern = "{}"
			}
		} else {
			buf, _ := json.Marshal(fmap)
			str := string(buf)
			re := regexp.MustCompile(`{(.*):{"\$regularExpression":{"options":"(\S+)?","pattern":"(\^)?(\S+)"}}}`)
			stat.QueryPattern = re.ReplaceAllString(str, "{$1:/$3.../$2}")
		}
	}
	if stat.Op == "" {
		return stat, nil
	}
	re := regexp.MustCompile(`\[1(,1)*\]`)
	stat.QueryPattern = re.ReplaceAllString(stat.QueryPattern, `[...]`)
	re = regexp.MustCompile(`\[{\S+}(,{\S+})*\]`) // matches repeated doc {"base64":1,"subType":1}}
	stat.QueryPattern = re.ReplaceAllString(stat.QueryPattern, `[...]`)
	re = regexp.MustCompile(`^{("\$match"|"\$sort"):(\S+)}$`)
	stat.QueryPattern = re.ReplaceAllString(stat.QueryPattern, `$2`)
	re = regexp.MustCompile(`^{("(\$facet")):\S+}$`)
	stat.QueryPattern = re.ReplaceAllString(stat.QueryPattern, `{$1:...}`)
	re = regexp.MustCompile(`{"\$oid":1}`)
	stat.QueryPattern = re.ReplaceAllString(stat.QueryPattern, `1`)
	if isGetMore {
		stat.Op = cmdGetMore
	}
	return stat, nil
}

func isRegex(doc map[string]interface{}) bool {
	if buf, err := json.Marshal(doc); err != nil {
		return false
	} else if strings.Contains(string(buf), "$regularExpression") {
		return true
	}
	return false
}

func getOp(command map[string]interface{}) string {
	for _, v := range ops {
		if command[v] != nil {
			return v
		}
	}
	return ""
}

func cb(value interface{}) interface{} {
	return 1
}