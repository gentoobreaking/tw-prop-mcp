package domain

import "time"

// PingToSqm is the conversion factor: 1 坪 = 3.305785 平方公尺.
const PingToSqm = 3.305785

// Transaction is the domain model for the transaction table.
// Follows DATA_MODEL.md and SPEC.md. Immutable artifact per P2.
type Transaction struct {
	ID                string    `json:"id"`
	SnapshotID        string    `json:"snapshot_id"`
	ImportBatchID     string    `json:"import_batch_id,omitempty"`
	TransactionID     string    `json:"transaction_id"`
	TransactionDate   time.Time `json:"transaction_date"`
	TransactionType   string    `json:"transaction_type,omitempty"`
	County            string    `json:"county"`
	District          string    `json:"district"`
	Section           string    `json:"section"`
	LandNumber        string    `json:"land_number"`
	TransactionTarget string    `json:"transaction_target,omitempty"`
	TotalPrice        int64     `json:"total_price"`
	UnitPrice         int64     `json:"unit_price"`
	LandAreaSqm       float64   `json:"land_area_sqm,omitempty"`
	BuildingAreaSqm   float64   `json:"building_area_sqm,omitempty"`
	UrbanZoning       string    `json:"urban_zoning,omitempty"`
	NonUrbanZoning    string    `json:"non_urban_zoning,omitempty"`
	LandUseCategory   string    `json:"land_use_category,omitempty"`
	BuildingType      string    `json:"building_type,omitempty"`
	Floor             string    `json:"floor,omitempty"`
	Age               int       `json:"age,omitempty"`
	ParkingAreaSqm    float64   `json:"parking_area_sqm,omitempty"`
	ParkingPrice      int64     `json:"parking_price,omitempty"`
	SourceRecordHash  string    `json:"source_record_hash"`
	CreatedAt         time.Time `json:"created_at,omitempty"`
}

// PricePerPing returns unit price converted to 元/坪.
// Formula: unit_price (元/㎡) * 3.305785
func (t *Transaction) PricePerPing() float64 {
	return float64(t.UnitPrice) * PingToSqm
}

// SqmFromPing converts ping to sqm.
func SqmFromPing(ping float64) float64 {
	return ping * PingToSqm
}

// PingFromSqm converts sqm to ping.
func PingFromSqm(sqm float64) float64 {
	return sqm / PingToSqm
}
