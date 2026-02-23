# Prosemark Binder Conformance Suite

This directory contains the language-agnostic conformance suite for the Prosemark Binder format. Any implementation claiming Prosemark Binder compatibility SHOULD pass the applicable test categories in this suite.

## Purpose

The conformance suite provides:

- **Fixture-based test cases** that encode normative behavior from the specification
- **JSON Schemas** that validate the structure of parser and operation outputs
- **A runner contract** that any implementation can use to wire up conformance testing

## Structure

```
conformance/
├── README.md               # This file
└── v1/                     # Tests for Prosemark Binder format v1
    ├── README.md           # v1 fixture authoring guide
    ├── runner-contract.md  # Normative runner protocol
    ├── diagnostic-codes.md # Full diagnostic code registry
    ├── schema/             # JSON Schemas for inputs and outputs
    ├── parse/              # Parse-domain fixtures
    └── ops/                # Operation-domain fixtures
```

## Versioning

Each subdirectory corresponds to a major format version. When a breaking change is introduced to the specification, a new versioned directory is added (e.g., `v2/`). Older versions remain to support implementations targeting earlier formats.

## Running the Suite

See `v1/runner-contract.md` for the normative runner protocol, which defines:

- Required inputs for each fixture type
- Execution steps
- Pass/fail determination rules

Suggested justfile targets for CI integration:

```
just conformance-validate   # JSON Schema validate all fixture files
just conformance-run        # Run fixtures against a local implementation binary
just conformance-generate   # Regenerate expected outputs from reference implementation
```

## Contributing Fixtures

1. Choose the correct domain: `v1/parse/fixtures/` or `v1/ops/fixtures/<operation>/`
2. Follow naming convention: `NNN-kebab-slug` (three-digit zero-padded prefix)
3. Validate all JSON files against the schemas in `v1/schema/` before committing
4. See `v1/README.md` for detailed authoring guidance

## Relationship to Specs

The conformance suite is normatively derived from:

- `docs/prosemark_binder_format_spec_v1.md` — parse behavior
- `docs/prosemark_binder_operations_spec_v1.md` — mutation behavior
