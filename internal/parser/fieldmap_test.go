package parser

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestFieldMap(t *testing.T) {
	p := NewParser()
	file, _ := os.Open("../../tests/testdata/sample.csv")
	defer file.Close()
	
	rows, err := p.ParseCSV(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	
	for i, row := range rows {
		fmt.Printf("Row %d:\n", i)
		for k, v := range row {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
}
