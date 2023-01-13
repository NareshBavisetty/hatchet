// Copyright 2022-present Kuei-chun Chen. All rights reserved.

package hatchet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// htmlHandler responds to API calls
func htmlHandler(w http.ResponseWriter, r *http.Request) {
	/** APIs
	 * /tables/{table}
	 * /tables/{table}/charts/slowops
	 * /tables/{table}/logs
	 * /tables/{table}/logs/slowops
	 * /tables/{table}/stats/slowops
	 */
	htmlPrefix := "/tables/"
	tokens := strings.Split(r.URL.Path[len(htmlPrefix):], "/")
	var tableName, category, attr string
	for i, token := range tokens {
		if i == 0 {
			tableName = token
		} else if i == 1 {
			category = token
		} else if i == 2 {
			attr = token
		}
	}

	if tableName == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": "missing table name"})
		return
	} else if category == "" && attr == "" { // default to /table/{table}/stats/slowops
		category = "stats"
		attr = "slowops"
	}

	if category == "charts" && attr == "ops" {
		summary := getTableSummary(tableName)
		duration := r.URL.Query().Get("duration")
		chartType := "ops"
		var start, end string
		docs, err := getAverageOpTime(tableName, duration)
		if len(docs) > 0 {
			start = docs[0].Date
			end = docs[len(docs)-1].Date
		}
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		templ, err := GetChartTemplate(attr, "chartType")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		doc := map[string]interface{}{"Table": tableName, "OpCounts": docs, "Title": "Average Operation Time",
			"Summary": summary, "Attr": attr, "Chart": chartType, "Start": start, "End": end, "VAxisLabel": "seconds"}
		if err = templ.Execute(w, doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		return
	} else if category == "charts" && attr == "slowops" {
		summary := getTableSummary(tableName)
		chartType := r.URL.Query().Get("type")
		duration := r.URL.Query().Get("duration")
		if chartType == "" || chartType == "stats" {
			chartType = "stats"
			var start, end string
			docs, err := getSlowOpsCounts(tableName, duration)
			if len(docs) > 0 {
				start = docs[0].Date
				end = docs[len(docs)-1].Date
			}
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			templ, err := GetChartTemplate(attr, chartType)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			doc := map[string]interface{}{"Table": tableName, "OpCounts": docs, "Title": "Slow Operation Counts",
				"Summary": summary, "Attr": attr, "Chart": chartType, "Start": start, "End": end, "VAxisLabel": "count"}
			if err = templ.Execute(w, doc); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			return
		} else if chartType == "counts" {
			docs, err := getOpsCounts(tableName)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			templ, err := GetChartTemplate("pieChart", chartType)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			doc := map[string]interface{}{"Table": tableName, "NameValues": docs, "Title": "Ops Counts",
				"Summary": summary, "Attr": attr, "Chart": chartType}
			if err = templ.Execute(w, doc); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			return
		}
	} else if category == "charts" && attr == "connections" {
		summary := getTableSummary(tableName)
		chartType := r.URL.Query().Get("type")
		duration := r.URL.Query().Get("duration")
		if chartType == "" || chartType == "accepted" {
			chartType = "accepted"
			docs, err := getAcceptedConnsCounts(tableName)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			templ, err := GetChartTemplate("pieChart", chartType)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			doc := map[string]interface{}{"Table": tableName, "NameValues": docs, "Title": "Accepted Connections",
				"Summary": summary, "Attr": attr, "Chart": chartType}
			if err = templ.Execute(w, doc); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			return
		} else { // type is time or total
			docs, err := getConnectionStats(tableName, chartType, duration)
			var start, end string
			if len(docs) > 0 {
				start = docs[0].IP
				end = docs[len(docs)-1].IP
			}
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			templ, err := GetChartTemplate(attr, chartType)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			if len(docs) == 0 {
				docs = []Remote{{IP: "No data", Accepted: 0, Ended: 0}}
			}
			doc := map[string]interface{}{"Table": tableName, "Remote": docs,
				"Summary": summary, "Attr": attr, "Chart": chartType, "Start": start, "End": end}
			if err = templ.Execute(w, doc); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
				return
			}
			return
		}
	} else if category == "logs" && attr == "slowops" {
		topN := ToInt(r.URL.Query().Get("topN"))
		if topN == 0 {
			topN = 25
		}
		logstrs, err := getSlowestLogs(tableName, topN)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		summary := getTableSummary(tableName)
		templ, err := GetLogTableTemplate(attr)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		doc := map[string]interface{}{"Table": tableName, "Logs": logstrs, "Summary": summary}
		if err = templ.Execute(w, doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		return
	} else if category == "stats" && attr == "slowops" {
		collscan := false
		if r.URL.Query().Get("COLLSCAN") == "true" {
			collscan = true
		}
		var order, orderBy string
		orderBy = r.URL.Query().Get("orderBy")
		if orderBy == "" {
			orderBy = "avg_ms"
		} else if orderBy == "index" || orderBy == "_index" {
			orderBy = "_index"
		}
		order = r.URL.Query().Get("order")
		if order == "" {
			if orderBy == "op" || orderBy == "ns" {
				order = "ASC"
			} else {
				order = "DESC"
			}
		}
		summary := getTableSummary(tableName)
		ops, err := getSlowOps(tableName, orderBy, order, collscan)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		templ, err := GetStatsTableTemplate(collscan, orderBy)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		doc := map[string]interface{}{"Table": tableName, "Ops": ops, "Summary": summary}
		if err = templ.Execute(w, doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		return
	} else if category == "logs" && attr == "" {
		tableName := tokens[0]
		component := r.URL.Query().Get("component")
		context := r.URL.Query().Get("context")
		severity := r.URL.Query().Get("severity")
		duration := r.URL.Query().Get("duration")
		logs, err := getLogs(tableName, fmt.Sprintf("component=%v", component),
			fmt.Sprintf("context=%v", context), fmt.Sprintf("severity=%v", severity), fmt.Sprintf("duration=%v", duration))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		templ, err := GetLogTableTemplate(attr)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		summary := getTableSummary(tableName)
		doc := map[string]interface{}{"Table": tableName, "Logs": logs, "LogLength": len(logs),
			"Summary": summary, "Context": context, "Component": component, "Severity": severity}
		if err = templ.Execute(w, doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": 0, "error": err.Error()})
			return
		}
		return
	}
}

func getSlowOpsStats(tableName string, orderBy string) ([]byte, error) {
	if orderBy == "" {
		orderBy = "avg_ms"
	}
	ops, err := getSlowOps(tableName, orderBy, "DESC", false)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ops)
}
