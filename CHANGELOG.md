# Change Log

## [0.0.26] - 2022-11-16

### Added

- The new `draft info` command from #157 prints supported language and field information in json format.
- An integration test was added for the installation shell script to better ensure that the script works as expected.

### Fixed

- File path output for root locations had a bug with string-formatted paths. The `path.Join` method has been substituted to fix this.

### Changed

- Remaining uses of the `viper` library have been migrated to `gopkg.in/yaml.v3` for consistency.
- Unused files in the `web` package have been removed.
- Minor reorganization across the `config` and `addons` packages to reduce the number of exported functions and types.