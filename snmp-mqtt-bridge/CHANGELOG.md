# Changelog

All notable changes to the SNMP-MQTT Bridge Home Assistant app are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- Load MQTT username and password from Home Assistant app options during startup, so automatic reconnects authenticate correctly after an update or restart.

### Added

- Validate in CI that the app version has a matching changelog entry.

## [0.0.16] - 2026-07-15

### Added

- Redesigned the dashboard with live availability, outlet, current, power, and device summaries.
- Added MQTT connection status and a persistent Safe Mode for device control confirmations.
- Added device search, inline enable controls, device icons, and live status summaries.

### Fixed

- Prevent MQTT connection attempts and frontend requests from hanging indefinitely during broker outages.
- Restore command subscriptions and Home Assistant discovery after MQTT reconnects.

## [0.0.15] - 2026-07-04

### Fixed

- Normalize temperature units in Home Assistant MQTT discovery payloads.

## [0.0.14] - 2026-06-18

### Fixed

- Republish Home Assistant discovery after MQTT settings are changed or the connection is restored.

## [0.0.13] - 2026-06-18

### Fixed

- Keep MQTT discovery and command subscriptions synchronized when devices are added, updated, enabled, disabled, or removed.

## [0.0.12] - 2026-06-10

### Added

- Added Home Assistant app icon and logo assets.
- Expanded MQTT command handling test coverage.

### Fixed

- Match SNMP traps to devices configured with hostnames.
- Bind the Home Assistant ingress path correctly from the app environment.

## [0.0.11] - 2026-06-09

### Fixed

- Reject failed SNMP SET responses instead of reporting outlet commands as successful.

## [0.0.10] - 2026-06-09

### Fixed

- Handle profile-specific outlet state values and ATEN outlet control settings.

## [0.0.9] - 2026-05-27

### Added

- Added the ATEN PE8108G eco PDU profile with outlet control and per-outlet energy counters.

### Changed

- Refined device details, profile selection, status badges, and the dark frontend theme.

### Fixed

- Serialize WebSocket writes and improve local device state display.

## [0.0.8] - 2026-03-23

### Added

- Publish bridge self-discovery to Home Assistant.

### Fixed

- Reflect MQTT credentials supplied by Home Assistant in the settings interface.
- Format PostgreSQL connection ports correctly.

## [0.0.7] - 2026-02-01

### Fixed

- Use relative frontend asset paths for Home Assistant ingress compatibility.

## [0.0.6] - 2026-02-01

### Fixed

- Use a pure-Go SQLite driver for CGO-free builds.

## [0.0.5] - 2026-02-01

### Fixed

- Publish Home Assistant app container packages as public images.

## [0.0.4] - 2026-02-01

### Fixed

- Correct the Home Assistant app image path format.

## [0.0.3] - 2026-02-01

### Added

- Added the Home Assistant app with ingress and Bashio-based MQTT service integration.

## [0.0.2] - 2026-02-01

### Added

- Added Energenie EG-PDU-003 support with outlet control, power metrics, and Home Assistant entities.

## [0.0.1] - 2026-02-01

### Added

- Initial release with APC ATS and PDU profiles, SNMP polling, traps, MQTT, and Home Assistant discovery.
