"""Exact reviewed Q47 helper transition; every other baseline pin stays frozen."""
PRODUCT_CHANGE = {
    "schemaVersion": 1,
    "campaign": "Q47",
    "path": "apps/player/docs/assets/js/platform-install.js",
    "original": {"gitBlob": "e778290caacb567cc67384fda07a4f2b40272833",
                 "bytes": 6078, "sha256": "ebb1508c7b103e61eab604d6e2a437f43f16180dc5f05d3660506a05fae20c1a"},
    "current": {"gitBlob": "e5e44fe9c8e84c221c38f959a1282923e0b11678",
                "bytes": 8948, "sha256": "4a9160045fd1090a64096f9a18efbb6b8da162303f3daf900e4a615919e2c985"},
    "beforeProof": {"run": 37339459348, "artifact": 11357404726,
                    "head": "de5e96cd3a4b31c0018447733bab8b36e634a245",
                    "tree": "4deb50d8c27e4403fd366e36122e8be0a8b7a932",
                    "archiveBytes": 497943,
                    "archiveSHA256": "deaed7f80c35b3e11576add6f4d868367b769347ef3dc91fabddb2c643c4b304",
                    "classification": "deadline-contract-red", "repeats": 2, "casesPerRepeat": 2,
                    "rootAdmitted": True, "independentlyAdmitted": True},
    "scope": "Whole fetch/body deadline, retry/fallback, cancellation and stale UI guards.",
    "remainingAcceptance": "Fresh primary and supplemental successes plus protected checks and ancestry.",
}
APPROVED_PRODUCT_BLOBS = {PRODUCT_CHANGE["path"]: PRODUCT_CHANGE["current"]["gitBlob"]}
