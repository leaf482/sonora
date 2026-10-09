# Licensed audio feasibility — Task 1

Reviewed 2026-10-09 by Codex Dev. **Fixture rights verified for the three manifest entries; independent review pending.** No blocking licensing uncertainty was identified for local storage, playback and redistribution of these identified fixtures under their stated licenses and conditions. This is evidence-based verification of published permissions, not a warranty of ownership or a completed production release gate.

## Scope and decision

Selected [Pong](https://opengameart.org/content/pong-0) by yesme (CC0 1.0), and [Gustav Sting](https://incompetech.com/music/royalty-free/index.html?isrc=USUAN1100529) / [Disco Sting](https://incompetech.com/music/royalty-free/index.html?isrc=USUAN1100363) by Kevin MacLeod (CC BY 4.0). They provide three small, complete MP3 frame sequences without ripping, accounts or paid licenses. See the [manifest](../fixtures/audio/manifest.json) for exact download URLs, timestamps, original/final hashes, changes, evidence references and permitted uses; see [fixture credits and procedures](../fixtures/audio/README.md) for distribution notices and reproducible acquisition.

The reviewed usage is a Sonora development repository/fixture archive, local copies and local browser testing, with the notices retained. CC0 and CC BY also support public/commercial sharing within the rights granted, but this task does not launch or clear every operational requirement for a production service. Artwork is omitted. No DB/API/UI/storage service was added; the Go/PostgreSQL/Next.js/filesystem architecture is unchanged.

## Candidate research

- **Jamendo:** the [artist licensing explanation](https://support-artist.jamendo.com/creative-commons-explained) describes artist-selected combinations. No individual Jamendo track was approved or downloaded. A platform-wide policy is insufficient; provider access conditions and the exact track's license still need review.
- **FMA:** its [license guide](https://freemusicarchive.org/License_Guide) and [FAQ](https://freemusicarchive.org/faq/) describe several licenses, including restricted downloads. No FMA track was approved. Do not infer fixture redistribution from a download button.
- **Wikimedia Commons:** inspected [Palm Leaf Rag](https://commons.wikimedia.org/wiki/File:Scott_Joplin_-_Palm_Leaf_Rag_(1903).ogg). The page distinguishes composition status from the performance's CC BY-SA 3.0 notice. It was not selected: the fixture set favors CC0/CC BY and smaller files. The site's CC0 structured-data footer is not the recording's license.
- **Creator-posted CC0:** the specific [Pong submission](https://opengameart.org/content/pong-0) names yesme, identifies their game music, offers the attached MP3 and links CC0 1.0. That attachment alone is approved here, not all OpenGameArt content.
- **Other CC0 candidates:** [Pop Music](https://opengameart.org/content/pop-music) identifies Alex McCulloch / Pro Sensory and lists High/Forest under CC0. Original downloads had incomplete final MPEG frames. They were excluded for technical quality, not a finding of infringement; no copies remain in this repo.
- **Direct CC BY publisher:** Incompetech's two individual ISRC pages display Kevin MacLeod credits with CC BY 4.0. Live [catalog records](https://incompetech.com/music/royalty-free/pieces.json) bind each ISRC/title to its original filename; the page template binds that record to its license notice/download. Retained evidence records that method. These two assets are approved, not a blanket catalog ingestion policy.

## Permission rationale and conditions

The [CC0 1.0 legal code](https://creativecommons.org/publicdomain/zero/1.0/legalcode.en) waives the affirmer's copyright/related rights, with a fallback license. Its worldwide scope supports copying, playback, fixture redistribution and modification, including commercial use. Sonora keeps courtesy credit. The dedication supplies no third-party-rights warranty and no endorsement.

The [CC BY 4.0 legal code](https://creativecommons.org/licenses/by/4.0/legalcode.en) grants reproduction/sharing/adaptation rights; sharing includes public performance and communication. Applied to these creator-published recordings, this supports local copies, browser playback and redistribution of the fixture files, rather than just synchronization into a video. Retain creator/supplied notices, source/license links and change disclosures. Do not restrict recipients' licensed reuse. The license lasts for the licensed rights' term, subject to compliance; it does not clear unrelated rights or non-waivable royalties. The included legal texts govern; this summary does not replace them.

The fixture README supplies attribution now. Public playback later requires accessible track/player credits as described there. The CC BY files retain their audio and non-picture metadata; removal of their embedded pictures/padding is explicitly disclosed, with both source and resulting hashes. No audio excerpts, resampling or normalization were performed.

## Remaining boundaries

- Independent read-only PASS/FIX review is still pending; no reviewer verdict is invented.
- Full decode, browser play/seek and target-browser coverage await implementation tasks. Frame integrity is not an end-to-end playback result. These short fixtures do not validate every listening-history branch.
- Before public release, review actual audience/territories, distribution terms, credit visibility and any applicable non-waivable collection obligations. No paid streaming-service agreement or blanket legal clearance is claimed. New assets and changed uses need new review.
- Evidence files preserve selected observed facts, dates and brief excerpts; they are not provider-signed attestations. A credible conflicting claim reopens the decision. No artwork, source-page logos or unrelated catalog data are licensed by this fixture decision.
- Missing upstream URLs alone do not revoke an existing grant. A changed page also is not automatically revocation: CC0 is intended to be irrevocable, and CC BY section 6 permits a licensor to stop distributing without terminating compliant prior grants. Nevertheless, Sonora handles uncertainty conservatively as below.

## Withdrawal and availability procedure

1. On a credible rights complaint, conflicting license evidence, changed bytes, or unexplained source/evidence loss, record asset ID, date, reason and the observed evidence in this document. Set that manifest asset's status to withdrawn (or review_required for an availability investigation). Do not reacquire, redistribute or load it while under review.
2. Preserve the historical source URLs, original/final hashes, evidence and decision record. Remove the audio file from active fixture distributions after recording the decision; retain its manifest entry as a non-included tombstone. Verification/reconstruction commands intentionally process only included assets. Do not silently replace it with an unrelated file at the same path.
3. Before any future backend exists, withdrawal is a repository/fixture packaging action. Once implemented, additionally mark Track/AudioAsset unavailable before deleting storage, reject new streams/ranges and retain collection/history placeholders per [DOMAIN_MODEL.md](DOMAIN_MODEL.md). In-flight or already delivered bytes cannot be recalled.
4. Determine whether a dead URL is only an availability issue or whether the permission itself is disputed. Consult the retained grant and obtain clarification as needed. Do not claim an irrevocable license vanished merely because a page disappeared.
5. Review affected archives/deployments/backups and arrange removal or restricted evidence retention as appropriate. Never rewrite Git history or destroy evidence automatically; that needs a separate scoped user decision. Existing downstream copies cannot be guaranteed erased.
6. Restore included status only after a documented rights/identity decision and fresh integrity checks, with independent review. A replacement recording gets its own identity, hashes, license evidence and attribution.

## Verification recorded

Three included files total 1,626,339 bytes. Original and prepared SHA-256 values are in the manifest; five retained evidence/license files are also hash-pinned. File structure checks found complete MPEG-1 Layer III frame sequences in the selected files. Reacquisition, preparation, manifest checks, links and Git checks are recorded in the task handoff; these are fixture checks, not application acceptance.
