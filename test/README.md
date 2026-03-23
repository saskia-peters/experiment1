# Test Directory

This directory contains unit tests and integration tests for the Jugendolympiade application.

## Test Files

### `input_test.go`
Comprehensive tests for Excel file import functionality.

#### Test Coverage

##### Integration Tests for `ReadXLSXFile`:
- ✅ **TestReadXLSXFile_ValidFile** - Tests reading a valid Excel file with correct headers and data
- ✅ **TestReadXLSXFile_EmptySheet** - Tests error handling for empty sheets
- ✅ **TestReadXLSXFile_OnlyHeader** - Tests error handling for sheets with only headers
- ✅ **TestReadXLSXFile_InvalidHeaders** - Tests validation of incorrect column headers
- ✅ **TestReadXLSXFile_InvalidAge** - Tests validation of invalid age values (non-numeric)
- ✅ **TestReadXLSXFile_InsufficientColumns** - Tests error handling for rows with too few columns
- ✅ **TestReadXLSXFile_EmptyRows** - Tests handling of empty rows (should be skipped gracefully)
- ✅ **TestReadXLSXFile_FileNotFound** - Tests error handling for non-existent files

##### Integration Tests for `ReadStationsFromXLSX`:
- ✅ **TestReadStationsFromXLSX_ValidFile** - Tests reading valid station data
- ✅ **TestReadStationsFromXLSX_NoStationsSheet** - Tests graceful handling when stations sheet is missing (optional)

##### Unit Tests (Skipped - Private Functions):
- **TestValidateHeaders** - Tests header validation logic (skipped as function is not exported)
- **TestValidateParticipantRow** - Tests participant row validation logic (skipped as function is not exported)

### `distribution_test.go`
Unit tests for the group distribution algorithm logic.

#### Test Coverage

- ✅ **TestDistribution_EmptyInput** - Tests handling of empty participant list
- ✅ **TestDistribution_SingleParticipant** - Tests distribution with one participant
- ✅ **TestDistribution_ExactlyMaxGroupSize** - Tests distribution with exactly 8 participants (one full group)
- ✅ **TestDistribution_MoreThanMaxGroupSize** - Tests distribution requiring multiple groups (9 participants)
- ✅ **TestDistribution_TwentyFourParticipants** - Tests realistic scenario with 24 participants (3 groups of 8)
- ✅ **TestDistribution_GroupSizeLimit** - Tests that no group exceeds MaxGroupSize (50 participants)
- ✅ **TestDistribution_StatisticsTracking** - Tests that Ortsverband, Geschlecht counts and AlterSum are correctly tracked
- ✅ **TestDistribution_GroupIDsSequential** - Tests that GroupIDs are sequential starting from 1

## Running Tests

### Run all tests:
```bash
cd test
go test -v
```

### Run from project root:
```bash
go test -v ./test/
```

### Run specific test:
```bash
go test -v -run TestReadXLSXFile_ValidFile
```

### Run with coverage:
```bash
go test -v -cover
```

### Generate coverage report:
```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Data

Tests create temporary Excel files dynamically using the `excelize` library. No static test files are required.

## Expected Behavior

### Valid Excel File Format:
- **Sheet name**: "Teilnehmende"
- **Headers**: Name, Ortsverband, Alter, Geschlecht (case-insensitive)
- **Data validation**:
  - Name: Required (rows with empty names are skipped)
  - Ortsverband: Optional (warning logged if missing)
  - Alter: Optional, must be numeric 0-150 if provided
  - Geschlecht: Optional, accepts M/W/D/männlich/weiblich/divers (warnings for unusual values)

### Stations (Optional):
- **Sheet name**: "Stationen"
- **Headers**: Station
- If missing, application continues without error

## Adding New Tests

When adding new test cases:

1. Follow the naming convention: `Test<FunctionName>_<Scenario>`
2. Use table-driven tests for multiple similar test cases
3. Use `t.TempDir()` for temporary files to ensure cleanup
4. Add descriptive comments explaining what each test validates
5. Update this README with new test descriptions

## Future Improvements

- Export validation functions (`validateHeaders`, `validateParticipantRow`) to enable direct unit testing
- Add benchmark tests for large Excel files
- Add tests for concurrent file reads
- Add tests for database insertion logic
- Add tests for group distribution algorithm
