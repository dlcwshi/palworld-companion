using System.Text;
using System.Text.RegularExpressions;
using CUE4Parse.Compression;
using CUE4Parse.FileProvider;
using CUE4Parse.MappingsProvider;
using CUE4Parse.UE4.Assets.Exports.Engine;
using CUE4Parse.UE4.Assets.Objects;
using CUE4Parse.UE4.Objects.Core.i18N;
using CUE4Parse.UE4.Objects.UObject;
using CUE4Parse.UE4.Versions;

namespace PalworldCompanion.PalData;

public sealed class Cue4ParseDataSource(string gameDirectory, string? usmapPath, string? oodlePath, bool verbose) : IPalworldDataSource
{
    private const string MonsterPackage = "Pal/Content/Pal/DataTable/Character/DT_PalMonsterParameter";
    private const string SpecialPackage = "Pal/Content/Pal/DataTable/Character/DT_PalCombiUnique";
    private const string EnglishNamesPackage = "Pal/Content/L10N/en/Pal/DataTable/Text/DT_PalNameText_Common";
    private const string ChineseNamesPackage = "Pal/Content/L10N/zh-Hans/Pal/DataTable/Text/DT_PalNameText_Common";

    public Task<RawExtraction> ExtractAsync(CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        try
        {
            if (oodlePath is null)
                throw new ExtractionException("Pal-Windows.pak uses Oodle compression. Supply a local --oodle library; automatic Oodle downloads are disabled.");
            OodleHelper.Initialize(oodlePath);
            if (OodleHelper.Instance is null)
                throw new ExtractionException($"Could not initialize the local Oodle library: {oodlePath}");
            var manifest = GameInstallation.FindManifest(gameDirectory);
            var buildId = GameInstallation.ReadBuildId(manifest);
            var pakDirectory = Path.Combine(gameDirectory, "Pal", "Content", "Paks");
            using var provider = new DefaultFileProvider(pakDirectory, SearchOption.TopDirectoryOnly,
                new VersionContainer(EGame.GAME_UE5_1), StringComparer.OrdinalIgnoreCase);
            if (usmapPath is not null) provider.MappingsContainer = new FileUsmapTypeMappingsProvider(usmapPath);
            provider.Initialize();
            var mounted = provider.Mount();
            if (verbose) Console.Error.WriteLine($"Mounted {mounted} archive(s); indexed {provider.Files.Count} files.");
            if (provider.Files.Count == 0) throw new ExtractionException("Pal-Windows.pak mounted but exposed no files.");

            var defaultGameBytes = provider.SaveAsset("Pal/Config/DefaultGame.ini");
            var gameVersion = ReadGameVersion(defaultGameBytes);
            var monsterTable = LoadTable(provider, MonsterPackage);
            var specialTable = LoadTable(provider, SpecialPackage);
            var englishNames = ReadNames(LoadTable(provider, EnglishNamesPackage));
            var chineseNames = ReadNames(LoadTable(provider, ChineseNamesPackage));
            var pals = ReadPals(monsterTable, englishNames, chineseNames);
            var tribeToId = BuildTribeIndex(pals);
            var specials = ReadSpecials(specialTable, tribeToId);

            var sources = new List<SourceFingerprint>
            {
                new("steamapps/appmanifest_1623730.acf", SourceHasher.HashFile(manifest)),
                new("Pal/Config/DefaultGame.ini", SourceHasher.HashBytes(defaultGameBytes)),
                FingerprintPackage(provider, MonsterPackage), FingerprintPackage(provider, SpecialPackage),
                FingerprintPackage(provider, EnglishNamesPackage), FingerprintPackage(provider, ChineseNamesPackage)
            };
            if (usmapPath is not null) sources.Add(new("mapping.usmap", SourceHasher.HashFile(usmapPath)));
            sources.Add(new("oodle-library", SourceHasher.HashFile(oodlePath)));
            return Task.FromResult(new RawExtraction(gameVersion, buildId, pals, specials, sources));
        }
        catch (ExtractionException) { throw; }
        catch (Exception ex)
        {
            var mappingHint = usmapPath is null
                ? " The current package may use unversioned properties; supply a version-matched local --usmap file. No mapping is downloaded automatically."
                : " Verify that --usmap matches this exact Steam build.";
            throw new ExtractionException($"Failed to extract Palworld breeding resources: {ex.GetType().Name}: {ex.Message}.{mappingHint}", ex);
        }
    }

    private static UDataTable LoadTable(DefaultFileProvider provider, string packagePath) =>
        provider.LoadPackageObject<UDataTable>(packagePath + "." + packagePath[(packagePath.LastIndexOf('/') + 1)..]);

    private static string ReadGameVersion(byte[] defaultGameBytes)
    {
        var text = Encoding.UTF8.GetString(defaultGameBytes);
        var match = Regex.Match(text, "(?m)^ProjectVersion\\s*=\\s*([^\\r\\n]+)$", RegexOptions.CultureInvariant);
        if (!match.Success || string.IsNullOrWhiteSpace(match.Groups[1].Value))
            throw new ExtractionException("Pal/Config/DefaultGame.ini does not contain a non-empty ProjectVersion.");
        return match.Groups[1].Value.Trim();
    }

    private static IReadOnlyDictionary<string, string> ReadNames(UDataTable table)
    {
        var result = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var (key, row) in table.RowMap)
        {
            var text = row.GetOrDefault<FText?>("TextData")?.Text?.Trim();
            if (!string.IsNullOrWhiteSpace(text)) result[key.Text] = text;
        }
        return result;
    }

    private static IReadOnlyList<RawPal> ReadPals(UDataTable table,
        IReadOnlyDictionary<string, string> englishNames, IReadOnlyDictionary<string, string> chineseNames)
    {
        var result = new List<RawPal>();
        foreach (var (key, row) in table.RowMap)
        {
            if (!row.GetOrDefault("IsPal", false) || row.GetOrDefault("ZukanIndex", 0) <= 0 ||
                row.GetOrDefault("IsBoss", false) || row.GetOrDefault("IsTowerBoss", false) || row.GetOrDefault("IsRaidBoss", false)) continue;
            var id = key.Text;
            var tribe = EnumValue(row, "Tribe");
            var overrideName = row.GetOrDefault("OverrideNameTextID", new FName("None")).Text;
            var candidates = new[] { overrideName, "PAL_NAME_" + tribe, tribe, "PAL_NAME_" + id, id }
                .Where(value => !string.IsNullOrWhiteSpace(value) && value != "None").Distinct(StringComparer.Ordinal).ToArray();
            var nameKey = candidates.FirstOrDefault(candidate => englishNames.ContainsKey(candidate) && chineseNames.ContainsKey(candidate));
            if (nameKey is null) throw new ExtractionException($"{MonsterPackage}: {id} has no shared English/zh-Hans name row; candidates={string.Join(',', candidates)}");
            result.Add(new RawPal(id, tribe, row.GetOrDefault("ZukanIndex", 0),
                NullIfEmpty(row.GetOrDefault("ZukanIndexSuffix", string.Empty)), nameKey,
                chineseNames[nameKey], englishNames[nameKey], row.GetOrDefault("CombiRank", -1),
                row.GetOrDefault("CombiDuplicatePriority", -1), row.GetOrDefault("IgnoreCombi", false)));
        }
        if (result.Count == 0) throw new ExtractionException($"{MonsterPackage} produced no canonical Pal rows.");
        return result;
    }

    private static IReadOnlyDictionary<string, string> BuildTribeIndex(IReadOnlyList<RawPal> pals)
    {
        var result = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var group in pals.GroupBy(item => item.TribeId, StringComparer.Ordinal))
        {
            var ids = group.Select(item => item.Id).Distinct(StringComparer.Ordinal).ToArray();
            if (ids.Length != 1) throw new ExtractionException($"Tribe {group.Key} maps to multiple canonical Pal IDs: {string.Join(',', ids)}");
            result[group.Key] = ids[0];
        }
        return result;
    }

    private static IReadOnlyList<RawSpecialCombination> ReadSpecials(UDataTable table, IReadOnlyDictionary<string, string> tribeToId)
    {
        var result = new List<RawSpecialCombination>();
        foreach (var (key, row) in table.RowMap)
        {
            var tribeA = EnumValue(row, "ParentTribeA"); var tribeB = EnumValue(row, "ParentTribeB");
            if (!tribeToId.TryGetValue(tribeA, out var parentA) || !tribeToId.TryGetValue(tribeB, out var parentB))
                throw new ExtractionException($"{SpecialPackage}: {key.Text} references an unmapped parent tribe: {tribeA}, {tribeB}");
            var child = row.GetOrDefault("ChildCharacterID", new FName("None")).Text;
            result.Add(new(parentA, ParseGender(EnumValue(row, "ParentGenderA")), parentB,
                ParseGender(EnumValue(row, "ParentGenderB")), child));
        }
        return result;
    }

    private static string EnumValue(FStructFallback row, string property)
    {
        var value = row.GetOrDefault(property, new FName("None")).Text;
        return value.Contains("::", StringComparison.Ordinal) ? value[(value.LastIndexOf("::", StringComparison.Ordinal) + 2)..] : value;
    }

    private static RawGender ParseGender(string value) => value.ToLowerInvariant() switch
    { "none" or "any" or "both" => RawGender.Any, "male" => RawGender.Male, "female" => RawGender.Female,
        _ => throw new ExtractionException($"Unsupported EPalGenderType value: {value}") };

    private static SourceFingerprint FingerprintPackage(DefaultFileProvider provider, string packagePath)
    {
        var files = provider.SavePackage(packagePath).OrderBy(item => item.Key, StringComparer.Ordinal)
            .Select(item => new SourceFingerprint(item.Key.Replace('\\', '/'), SourceHasher.HashBytes(item.Value))).ToArray();
        return new(packagePath, SourceHasher.HashCanonical(files));
    }

    private static string? NullIfEmpty(string value) => string.IsNullOrWhiteSpace(value) ? null : value.Trim();
}
