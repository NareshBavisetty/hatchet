// Copyright 2022-present Kuei-chun Chen. All rights reserved.

package hatchet

import (
	"strings"
	"testing"
)

func TestToInt(t *testing.T) {
	input := "23"
	value := ToInt(input)
	if value != 23 {
		t.Fatal("expected", 23, "but got", value)
	}

	str := ""
	value = ToInt(str)
	if value != 0 {
		t.Fatal("expected", 0, "but got", value)
	}
}

func TestReplaceSpecialChars(t *testing.T) {
	value := "a_b_c_d_e_name"

	filename := "a-b.c d:e,name"
	fname := replaceSpecialChars(filename)
	if value != fname {
		t.Fatal("expected", value, "but got", fname)
	}
}

func TestGetHatchetName(t *testing.T) {
	filename := "mongod.log"
	length := len(filename) - len(".log") + TAIL_SIZE
	hatchetName := getHatchetName(filename)
	if len(hatchetName) != length {
		t.Fatal("expected", length, "but got", len(hatchetName))
	}

	filename = "mongod.log.gz"
	length = len(filename) - len(".log.gz") + TAIL_SIZE
	hatchetName = getHatchetName(filename)
	if len(hatchetName) != length {
		t.Fatal("expected", length, "but got", len(hatchetName))
	}

	filename = "mongod"
	length = len(filename) + TAIL_SIZE
	hatchetName = getHatchetName(filename)
	if len(hatchetName) != length {
		t.Fatal("expected", length, "but got", len(hatchetName))
	}

	filename = "filesys-shard-00-01.abcde.mongodb.net_2021-07-24T10_12_58_2021-07-25T10_12_58_mongodb.log.gz"
	length = len(filename) + TAIL_SIZE
	hatchetName = getHatchetName(filename)
	modified := "filesys_shard_00_01_abcde_mongodb_net_2021_07_24T10_12_58_2021_07_25T10_12_58_mongodb"
	if !strings.HasPrefix(hatchetName, modified) {
		t.Fatal(modified+"_*", length, "but got", hatchetName)
	}
}