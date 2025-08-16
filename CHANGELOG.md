# Changelog

## [v0.0.6] - 2025-08-16

### Fixed
- Fix streaming connection issues by using buffered writer consistently
- Ensure proper flushing after each chunk write to prevent connection drops

### Added
- Comprehensive test suite for all handlers
- Streaming functionality tests including:
  - Chunked encoding verification
  - Content-Length header absence verification
  - Message flush immediacy tests
  - Connection reuse tests
  - Multiple message delivery tests

### Changed
- Code formatted with gofmt for consistency

## [v0.0.5] - Previous releases

See git history for details of previous releases.