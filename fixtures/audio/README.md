# Licensed audio fixtures

Three short music fixtures (1,626,339 audio bytes, about 1.55 MiB), prepared on 2026-10-09. [manifest.json](manifest.json) is the exact-file inventory; [research and withdrawal policy](../../docs/LICENSED_AUDIO.md) records the decision and limitations. Paths in the manifest are relative to this directory. The local [.gitattributes](.gitattributes) keeps hash-pinned text in LF form across checkouts and prevents MP3 text conversion.

These are development fixtures, not a production catalog or permission to ingest other provider assets. Carry this README, manifest, evidence, and license texts with any fixture archive/repository distribution. Project/code licensing must not override these audio licenses.

## Credits

- **Pong — yesme.** [Original submission](https://opengameart.org/content/pong-0), [CC0 1.0 Universal](https://creativecommons.org/publicdomain/zero/1.0/). No modifications. Courtesy attribution; the dedication does not require credit.
- **Gustav Sting — Kevin MacLeod (incompetech.com).** [Original track / ISRC USUAN1100529](https://incompetech.com/music/royalty-free/index.html?isrc=USUAN1100529), [Creative Commons Attribution 4.0 International](https://creativecommons.org/licenses/by/4.0/). Changes: embedded picture and ID3 padding removed; audio and remaining tag frames unchanged; local filename normalized.
- **Disco Sting — Kevin MacLeod (incompetech.com).** [Original track / ISRC USUAN1100363](https://incompetech.com/music/royalty-free/index.html?isrc=USUAN1100363), [Creative Commons Attribution 4.0 International](https://creativecommons.org/licenses/by/4.0/). Changes: embedded picture and ID3 padding removed; audio and remaining tag frames unchanged; local filename normalized.

License texts: [CC0](licenses/CC0-1.0.txt), [CC BY 4.0](licenses/CC-BY-4.0.txt). Works are supplied without warranties under these instruments. No creator endorses Sonora. No separate artwork is included.

For future playback, show the title/creator and an obvious Credits link from track details and the player. Credits must include the source, exact license link, and modifications above. A hidden developer README alone is insufficient for the intended public UI. Downloadable fixture bundles must retain these notices; do not add DRM or terms restricting the licensed reuse of CC BY audio.

## Evidence and technical limits

Each asset references a retained evidence JSON with the live source URL, observation time, license link and asset identity. These are selected factual records/excerpts, not full HTML archives, notarized ownership proof or legal opinions. Incompetech's detail pages assemble track-specific notices from their public catalog; records explain that binding.

SHA-256 is recorded for each original download and included fixture. MP3 frames were inspected to EOF: Pong 585 frames / 48kHz stereo; Gustav Sting 699 frames / 44.1kHz stereo; Disco Sting 629 frames / 44.1kHz stereo. Frame-derived durations (14.040, 18.260, 16.431 seconds) are estimates before decoder delay/padding, not authoritative playback durations. Audio payload hashes for the edited files prove no audio transcoding/trimming. Only embedded pictures and padding were omitted to avoid introducing separate artwork permission assumptions; other supplied metadata remains intact.

No full audio decode, browser playback, seek, or cross-browser acceptance was tested in this task. All fixtures are shorter than 30 seconds; they exercise the half-duration history threshold later, not the 30-second branch. A separately licensed longer sample or rights-documented synthetic test audio will be needed for that branch. No application was implemented.

## Verify the included files

Run from the repository root in PowerShell. This checks all three audio files and five retained evidence/license files. The manifest and this README are ordinary reviewable source documents, not self-hashed files.

```powershell
$ErrorActionPreference = 'Stop'
$fixtureRoot = (Resolve-Path -LiteralPath 'fixtures/audio').Path
$manifest = Get-Content -Raw -LiteralPath "$fixtureRoot/manifest.json" | ConvertFrom-Json
$records = @($manifest.assets | Where-Object status -eq 'included') + @($manifest.retained_files)
foreach ($record in $records) {
    $path = Join-Path $fixtureRoot $record.path
    if (!(Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing: $path" }
    if ((Get-Item -LiteralPath $path).Length -ne $record.bytes) { throw "Size: $path" }
    $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $record.sha256) { throw "SHA-256: $path" }
}
"PASS: $($records.Count) manifest files verified"
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }
git status --short
```

Git diff does not check untracked files. Before the user commits, inspect them separately and check text whitespace/relative paths too. No staging is needed.

## Reproduce acquisition and preparation

1. Read [LICENSED_AUDIO.md](../../docs/LICENSED_AUDIO.md), then re-open every asset's individual evidence page and exact license. Confirm identity, license applicability and any current concerns. Do not treat a provider name or HTTP 200 as permission.
2. Use the exact download URLs in the manifest; do not substitute previews, search results, mirrors, re-encodes or another recording. The original hashes identify the acquired bytes.
3. After that review, run the snippet below from the repository root with Python 3 in PowerShell. It downloads to memory (maximum 2,000,001 bytes per file), verifies source hashes, removes only ID3v2.2 picture frames/padding where recorded, verifies final hashes, and writes to a fresh temporary directory. It does not overwrite the checkout, update the manifest, or publish anything.
4. If every hash matches, compare the temporary outputs to the existing fixtures or copy only missing files into this directory. Preserve the checked-in evidence/license records. If a URL fails or hash changes, stop: keep the existing evidence, investigate and review a new candidate rather than updating the expected hash blindly. Sources can change; reproducibility depends on continued access to the pinned bytes or this repository copy.
5. To recover a missing license text, use its manifest text_download_url; for CC BY remove trailing blank lines to one final newline as recorded in text_preparation, then validate against retained_files. Evidence JSON captures historical observations and cannot be recreated byte-for-byte from a later live page; recover it from Git or record a newly dated review.

```powershell
$restore = @'
import hashlib, json, tempfile, urllib.request
from pathlib import Path

manifest = json.loads(Path("fixtures/audio/manifest.json").read_text(encoding="utf-8"))
output = Path(tempfile.mkdtemp(prefix="sonora-audio-"))
for asset in manifest["assets"]:
    if asset["status"] != "included":
        continue
    with urllib.request.urlopen(asset["download_url"], timeout=30) as response:
        data = response.read(2_000_001)
    assert len(data) == asset["original_bytes"], asset["id"]
    assert hashlib.sha256(data).hexdigest() == asset["original_sha256"], asset["id"]
    if asset["preparation"] == "remove-id3v22-pic":
        assert data[:6] == b"ID3\x02\x00\x00"
        end = 10 + sum((b & 127) << (7 * (3-i)) for i, b in enumerate(data[6:10]))
        pos, removed, kept = 10, 0, []
        while pos + 6 <= end and data[pos] != 0:
            size = int.from_bytes(data[pos+3:pos+6], "big")
            stop = pos + 6 + size
            assert stop <= end
            if data[pos:pos+3] == b"PIC":
                removed += 1
            else:
                kept.append(data[pos:stop])
            pos = stop
        assert removed == 1 and not any(data[pos:end])
        audio = data[end:]
        assert hashlib.sha256(audio).hexdigest() == asset["mpeg_audio_sha256"]
        body = b"".join(kept)
        n = len(body)
        data = data[:6] + bytes((n >> 21 & 127, n >> 14 & 127, n >> 7 & 127, n & 127)) + body + audio
    else:
        assert asset["preparation"] == "none"
    assert len(data) == asset["bytes"], asset["id"]
    assert hashlib.sha256(data).hexdigest() == asset["sha256"], asset["id"]
    assert Path(asset["path"]).name == asset["path"]
    (output / asset["path"]).write_bytes(data)
    print("Verified:", asset["id"])
print("Reconstructed fixtures (existing checkout untouched):", output)
'@
$restore | python -
if ($LASTEXITCODE -ne 0) { throw 'Fixture reconstruction failed' }
```
