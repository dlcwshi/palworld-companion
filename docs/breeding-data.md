# Breeding data chain (phase 1)

Version 0.5.0-dev introduces an offline generation and validation chain only. It adds no breeding UI, backend endpoint, database table, deployment behavior, or runtime game-file access.

## Authority and inputs

The only gameplay-data authority is the user's legitimate local Palworld Windows client. The tool associates `--game-dir` with `steamapps/appmanifest_1623730.acf`, records its `buildid`, and reads:

| Output | Local source |
| --- | --- |
| `gameVersion` | `Pal/Config/DefaultGame.ini` `ProjectVersion` |
| IDs, dex and normal-breeding fields | `DT_PalMonsterParameter` |
| unique parents, genders and child | `DT_PalCombiUnique` |
| `nameEn` | `L10N/en/.../DT_PalNameText_Common` |
| `nameZh` | `L10N/zh-Hans/.../DT_PalNameText_Common` |
| `steamBuildId` | local Steam appmanifest |

`breedingPower` maps to `CombiRank`; `breedingOrder` maps to `CombiDuplicatePriority`. Boss, tower, raid and non-Pal rows are excluded. `IgnoreCombi=false` is `eligible`; an ignored Pal produced by a unique combination is `specialOnly`; other ignored rows are `excluded`.

`sourceHash` is SHA-256 over normalized extracted records and hashes of each source package, manifest, mapping and Oodle library.

## Offline and legal boundary

CUE4Parse 1.2.2 is locked as an Apache-2.0 development dependency. The repository does not include game PAKs, mappings, Oodle binaries, decrypted assets, third-party breeding tables or machine-translated names. The tool never invokes CUE4Parse network download helpers. Users must have the right to read local files and separately obtain any build-matched mapping or decompressor they are entitled to use. The production Companion does not read game PAKs and does not load CUE4Parse, Oodle, or `.usmap` files.

## Current local verification (2026-07-18)

The generator code chain is implemented. The detected client is Steam BuildID `24181527`; `Pal-Windows.pak` mounted and indexed 185003 files. The archive uses Oodle compression, but neither the client nor the local development environment contains an Oodle runtime, and no build-matched `.usmap` was found. Therefore no official `frontend/src/generated/breeding-data.json` has been created or committed. Fixture-backed generator tests and the independent JSON Schema/semantic validator prove transformation and rejection behavior without presenting fixtures as official data.

To close the current blocker, provide an entitled local Oodle library using a placeholder-based command such as `--oodle <LOCAL_OODLE_DLL>`. After decompression works, only provide `--usmap <MATCHING_LOCAL_USMAP>` if extraction reports unversioned-property or mapping errors; that mapping must match BuildID `24181527`. Then regenerate with a fixed `--generated-at` and run both validators and the full repository suite.
