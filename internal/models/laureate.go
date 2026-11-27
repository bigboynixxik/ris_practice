// Package models defines the stored records and application errors.
package models

// Laureate represents a person or organization in the dataset.
type Laureate struct {
	ID          string  `json:"id"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	Gender      string  `json:"gender"`
	Born        string  `json:"born"`
	BornCity    string  `json:"born_city"`
	BornCountry string  `json:"born_country"`
	CountryCode string  `json:"country_code"`
	Died        string  `json:"died"`
	DiedCity    string  `json:"died_city"`
	DiedCountry string  `json:"died_country"`
	DiedCode    string  `json:"died_country_code"`
	Prizes      []Prize `json:"prizes"`
}

// Prize represents an award received by a laureate.
type Prize struct {
	Year              int           `json:"year"`
	Category          string        `json:"category"`
	Share             int           `json:"share"`
	Motivation        string        `json:"motivation"`
	OverallMotivation string        `json:"overall_motivation"`
	Affiliations      []Affiliation `json:"affiliations"`
}

// Affiliation is the institution associated with a prize.
type Affiliation struct {
	Name    string `json:"name"`
	City    string `json:"city"`
	Country string `json:"country"`
}
