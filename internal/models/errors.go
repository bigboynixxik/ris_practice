package models

import "errors"

var (
	// ErrEmptyDataset indicates that a dataset contains no laureates.
	ErrEmptyDataset = errors.New("empty laureate dataset")
	// ErrInvalidLaureate indicates that a laureate has invalid identifying data.
	ErrInvalidLaureate = errors.New("invalid laureate")
	// ErrInvalidPrize indicates that a prize has invalid award data.
	ErrInvalidPrize = errors.New("invalid prize")
	// ErrDatasetMetadataMissing indicates that the dataset state row is missing.
	ErrDatasetMetadataMissing = errors.New("dataset metadata row is missing")
	// ErrLaureateNotFound indicates that an updated laureate no longer exists.
	ErrLaureateNotFound = errors.New("laureate not found")
)
