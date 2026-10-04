"""Validate selected, completed XCResult repetitions without native invocation."""

METHODS = {
    "PhotoAuthorizationJourneys": {
        "revokedPhotoDisappearsAfterSavedContentWasShown": {"401", "403", "404"},
        "connectionFailureKeepsTheSavedPhoto": set(),
    },
    "LibraryRefreshRecoveryJourneys": {
        "completeSavedLibraryOffersRetryAndPreservesTitles": set(),
    },
    "DownloadManagerRecoveryJourneys": {
        "diagnosedCatalogCorruptionKeepsResetBehindConfirmation": set(),
        "transientAutomaticDownloadFailureOffersRetryWithoutDeviceReset": set(),
    },
}


def selected_execution(tree, summary, suite, iterations):
    """Require every expected argument and exact completed repetition indices.

    Xcode 27's installed test-results schema uses Repetition nodes, including
    beneath Swift Testing Arguments. Method-level totals alone are insufficient.
    Unknown schemas and skipped or missing repetitions fail closed.
    """
    cases = []
    expected = METHODS[suite]

    def visit(node, selected=False):
        selected = selected or suite == node.get("name") or str(node.get("nodeIdentifier", "")).startswith(suite + "/")
        if selected and node.get("nodeType") == "Test Case":
            identifier = node.get("nodeIdentifier", "")
            method = next((m for m in expected if identifier.startswith(suite + "/" + m + "(")), None)
            if method:
                children = node.get("children", [])
                argument_nodes = [c for c in children if c.get("nodeType") == "Arguments"]
                groups = argument_nodes if argument_nodes else [node]
                runs = []
                for group in groups:
                    repetitions = [c for c in group.get("children", []) if c.get("nodeType") == "Repetition"]
                    runs.append({
                        "argument": group.get("name") if argument_nodes else None,
                        "result": group.get("result", "unknown"),
                        "repetitions": [{"index": c.get("nodeIdentifier"), "result": c.get("result", "unknown")}
                                        for c in repetitions],
                    })
                cases.append({"method": method, "result": node.get("result", "unknown"), "runs": runs})
        for child in node.get("children", []):
            visit(child, selected)

    for node in (tree or {}).get("testNodes", []):
        visit(node)
    indices = {str(i) for i in range(1, iterations + 1)}
    complete = bool(summary and summary.get("result") in {"Passed", "Failed"} and
                    summary.get("passedTests", 0) + summary.get("failedTests", 0) == len(expected) and
                    summary.get("skippedTests", 0) == 0 and len(cases) == len(expected) and
                    {case["method"] for case in cases} == set(expected))
    for case in cases:
        groups = case["runs"]
        arguments = expected[case["method"]]
        complete = complete and case["result"] in {"Passed", "Failed"} and bool(groups)
        complete = complete and (len(groups) == len(arguments) and {g["argument"] for g in groups} == arguments if arguments else
                                 len(groups) == 1 and groups[0]["argument"] is None)
        for group in groups:
            repetitions = group["repetitions"]
            complete = complete and group["result"] in {"Passed", "Failed"} and len(repetitions) == iterations
            complete = complete and {r["index"] for r in repetitions} == indices
            complete = complete and all(r["result"] in {"Passed", "Failed"} for r in repetitions)
    return cases, bool(complete)
