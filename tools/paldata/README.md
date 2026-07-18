# Offline Palworld breeding-data generator

This .NET 8 tool reads a user-owned local Windows Palworld installation with CUE4Parse 1.2.2. It never contacts Steam, a mapping service, a data API, or CUE4Parse's Oodle download helpers.

Inputs:

- `--game-dir`: client root containing `Palworld.exe` and `Pal/Content/Paks/Pal-Windows.pak`.
- `--oodle`: a locally supplied `oo2core_9_win64.dll` or compatible Oodle library.
- `--usmap`: optional local mapping matching the exact Steam build, used only when extraction reports unversioned-property or mapping errors.
- `--out`: destination JSON in an existing directory.

```powershell
dotnet run --project tools/paldata -- --game-dir <PALWORLD_CLIENT_DIR> --oodle <LOCAL_OODLE_DLL> --out frontend\src\generated\breeding-data.json --generated-at 2026-07-18T00:00:00Z --pretty
dotnet run --project tools/paldata -- --verify-only --out frontend\src\generated\breeding-data.json
```

If extraction then reports unversioned-property or mapping errors, rerun the generation command with `--usmap <MATCHING_LOCAL_USMAP>`.

The generator rejects missing names, invalid ranges, duplicate IDs, broken references, unstable ordering, duplicate/conflicting special pairs, and inconsistent normal-breeding classifications. Output is UTF-8 without BOM, LF-only, atomically replaced, and reproducible when `--generated-at` and source files are unchanged.

No `.usmap`, Oodle library, game archive, or generated intermediate is committed. The production Companion does not read PAK files or load CUE4Parse, Oodle, or mappings. See `docs/breeding-data.md` for source fields and the current verified build status.
