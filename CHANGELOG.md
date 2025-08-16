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

See git history for details of previous releases.Changelog
=========


(unreleased)
------------
- Add changelog for v0.0.6 release. [Stavros Korokithakis]

  Document fixes and improvements in this release including streaming
  connection fixes and comprehensive test suite addition.
- Add comprehensive test suite for streaming functionality. [Stavros
  Korokithakis]

  - Add tests for all handler functions
  - Test streaming with proper chunked encoding verification
  - Verify no Content-Length header in streaming responses
  - Test immediate message flushing after writes
  - Test connection reuse and multiple message delivery
  - Add test initialization to set up logging and cache
  - Format code with gofmt
- Fix streaming connection issues by using buffered writer consistently.
  [Stavros Korokithakis]

  Switch from direct connection writes to buffered writer for all chunk
  operations and ensure proper flushing after each write. This prevents
  premature connection closures caused by unflushed buffered data.
- Fix build issues and modernize Go project. [Stavros Korokithakis]

  - Initialize Go module (go.mod) for the project
  - Add required dependencies (gorilla/mux, go-cache, topic)
  - Replace deprecated ioutil.Discard with io.Discard
- Change default page copy. [Stavros Korokithakis]
- Merge pull request #1 from caarlos0/patch-1. [Stavros Korokithakis]

  Update goreleaser.yml
- Update goreleaser.yml. [Carlos Alexandro Becker]

  The property was renamed, check https://github.com/goreleaser/goreleaser#build-customization
- Add more documentation for the push-only functionality. [Stavros
  Korokithakis]
- Fix the name field in posted messages. [Stavros Korokithakis]
- Add goreleaser arch. [Stavros Korokithakis]
- Fix Goreleaser name. [Stavros Korokithakis]
- Add goreleaser. [Stavros Korokithakis]
- Add push-only feature. [Stavros Korokithakis]
- Add goxc. [Stavros Korokithakis]
- Add note about releases. [Stavros Korokithakis]
- Add blog post link. [Stavros Korokithakis]
- Change license to the AGPL. [Stavros Korokithakis]
- Change demo link. [Stavros Korokithakis]
- Don't buffer things. [Stavros Korokithakis]
- Add keepalives and connection termination. [Stavros Korokithakis]
- Add real-time, chunked HTTP streaming. [Stavros Korokithakis]
- Make cache atomic. [Stavros Korokithakis]
- Add demo. [Stavros Korokithakis]
- Improve format, add limit parameter. [Stavros Korokithakis]
- Set the proper headers. [Stavros Korokithakis]
- Change parameters to more sane values. [Stavros Korokithakis]
- Initial version. [Stavros Korokithakis]


