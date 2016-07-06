// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package sh

import "io/ioutil"

func Fuzz(data []byte) int {
	prog, err := Parse(data, "", ParseComments)
	if err != nil {
		return 0
	}
	Fprint(ioutil.Discard, prog)
	return 1
}
