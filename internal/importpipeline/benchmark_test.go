package importpipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/normalizer"
	"tw-prop-mcp/internal/parser"
	"tw-prop-mcp/internal/validator"
)

// moiSampleCSVHeader matches the official MOI real-price registration CSV format
// as published by data.gov.tw / lvr.land.moi.gov.tw (內政部不動產交易實價查詢).
const moiSampleCSVHeader = "鄉鎮市區,交易標的,土地位置建物門牌,土地移轉總面積平方公尺,都市土地使用分區,非都市土地使用分區,非都市土地使用編定,交易年月日,交易筆棟數,移轉層次,總樓層數,建物型態,主要用途,主要建材,建築完成年月,建物移轉總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,建物現況格局-隔間,有無管理組織,總價元,單價元平方公尺,車位類別,車位移轉總面積平方公尺,車位總價元,備註,編號,主建物面積,附屬建物面積,陽台面積,電梯,移轉編號"

// moiRealAddressSample generates realistic MOI CSV rows with real address patterns.
// Each row follows the official real-price registration data format.
func moiRealAddressSample(rows int) string {
	var sb strings.Builder
	sb.WriteString(moiSampleCSVHeader)
	sb.WriteString("\n")
	counties := []string{"臺北市", "臺中市", "臺南市", "高雄市", "新北市", "桃園市"}
	districts := []string{"中山區", "大安區", "信義區", "內湖區", "文山區"}
	sections := []string{"中山段", "大安段", "信義段", "內湖段", "文山段"}
	landNumbers := []string{"0001-0002", "0003-0004", "0005-0006", "0007-0008", "0009-0010"}

	for i := 0; i < rows; i++ {
		county := counties[i%len(counties)]
		district := districts[i%len(districts)]
		section := sections[i%len(sections)]
		landNum := landNumbers[i%len(landNumbers)]
		addr := fmt.Sprintf("%s%s%s一小段%s地號", county, district, section, landNum)
		sb.WriteString(fmt.Sprintf(
			"%s,土地,%s,%.1f,住,,商,111/06/15,土地1建物1車位1,5,10,華廈,住,鋼筋混凝土造,1100101,%.1f,2,2,1,無,有,%d,%d,無,0.0,0,,%d,%.1f,%.1f,%.1f,有,TRANS%08d\n",
			district, addr,
			33.3+float64(i%100), // land_area
			33.3+float64(i%100), // building_area
			8000000+i*5000,      // total_price
			8000+i*100,          // unit_price
			i,                  // record_id
			12.5+float64(i%50),   // main_building_area
			3.0+float64(i%10),    // auxiliary_building_area
			5.0,                // balcony_area
			i,                  // transaction_id
		))
	}
	return sb.String()
}

// writeSampleCSV writes a realistic MOI-format CSV file for benchmarking.
func writeSampleCSV(b *testing.B, csvData string) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "moi_sample.csv")
	if err := os.WriteFile(path, []byte(csvData), 0o644); err != nil {
		b.Fatalf("write csv: %v", err)
	}
	return path
}

// newBenchPipeline creates a minimal ImportPipeline for benchmarks.
func newBenchPipeline() *ImportPipeline {
	p := NewImportPipeline(PipelineConfig{
		DataProvider: "臺北市",
		SnapshotID:   "bench-snapshot",
	}, nil)
	p.Parser = parser.NewParser()
	return p
}

// BenchmarkParseRealMOI benchmarks CSV parsing of real MOI-format data.
func BenchmarkParseRealMOI(b *testing.B) {
	p := parser.NewParser()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseOfficialCSV(path)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkNormalizeRealMOI benchmarks normalization of real MOI-format rows.
func BenchmarkNormalizeRealMOI(b *testing.B) {
	n := normalizer.New()
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)

	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	// Enrich once before timing so normalize has valid county/section/land_number
	rows = pp.enrichRows(rows)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, row := range rows {
			_, err := n.NormalizeTransaction(row, "snapshot-bench")
			if err != nil {
				b.Fatalf("normalize failed: %v", err)
			}
		}
	}
}

// BenchmarkValidateRealMOI benchmarks validation of real MOI-format transactions.
func BenchmarkValidateRealMOI(b *testing.B) {
	v := validator.New(nil)
	n := normalizer.New()
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)

	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	rows = pp.enrichRows(rows)

	txns := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		txn, err := n.NormalizeTransaction(row, "snapshot-bench")
		if err != nil {
			continue
		}
		txns = append(txns, *txn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range txns {
			issues := v.ValidateTransaction(&txns[j])
			if v.HasBlockingErrors(issues) {
				b.Fatal("unexpected validation error")
			}
		}
	}
}

// BenchmarkEnrichRows benchmarks the enrichRows step that parses parcel_address
// and derives county from the data source configuration.
func BenchmarkEnrichRows(b *testing.B) {
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)
	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pp.enrichRows(rows)
	}
}

// BenchmarkPipelineFullStage benchmarks normalize + validate as a pipeline stage
// to measure end-to-end transformation throughput on real MOI-format data.
func BenchmarkPipelineFullStage(b *testing.B) {
	n := normalizer.New()
	v := validator.New(nil)
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)
	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	rows = pp.enrichRows(rows)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, row := range rows {
			txn, err := n.NormalizeTransaction(row, "snapshot-bench")
			if err != nil {
				continue
			}
			issues := v.ValidateTransaction(txn)
			if v.HasBlockingErrors(issues) {
				b.Fatal("unexpected validation error")
			}
		}
	}
}

// BenchmarkConcurrentNormalizeValidate benchmarks concurrent throughput of
// normalize + validate, simulating multi-source import parallelism.
func BenchmarkConcurrentNormalizeValidate(b *testing.B) {
	n := normalizer.New()
	v := validator.New(nil)
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(200)
	path := writeSampleCSV(b, csvData)
	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	rows = pp.enrichRows(rows)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, row := range rows {
				txn, err := n.NormalizeTransaction(row, "snapshot-bench")
				if err != nil {
					continue
				}
				v.ValidateTransaction(txn)
			}
		}
	})
}

// BenchmarkDeduplicateRealMOI benchmarks the deduplicate stage with real MOI data.
func BenchmarkDeduplicateRealMOI(b *testing.B) {
	p := NewImportPipeline(PipelineConfig{}, nil)
	n := normalizer.New()
	pp := newBenchPipeline()
	csvData := moiRealAddressSample(1000)
	path := writeSampleCSV(b, csvData)
	rows, err := pp.Parser.ParseOfficialCSV(path)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	rows = pp.enrichRows(rows)

	txns := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		txn, err := n.NormalizeTransaction(row, "snapshot-bench")
		if err != nil {
			continue
		}
		txns = append(txns, *txn)
	}
	parcels := make([]domain.Parcel, len(txns))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.deduplicate(txns, parcels)
	}
}

