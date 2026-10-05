#!/usr/bin/env python3
"""Strict ancillary attachment fields only; no metadata paths/content are admitted."""


def attachment_fields_valid(value):
    if type(value) is not dict:
        return False
    count, admission = value.get("privateRunnerAttachmentCount"), value.get("runnerAttachmentAdmission")
    if type(count) is not int or not 0 <= count <= 1 or type(admission) is not str:
        return False
    if admission == "none":
        return count == 0
    if admission == "rejected":
        return value.get("outcome") == "incomplete" and value.get("failure") == "unclassified"
    return (admission == "recognized" and count == 1 and value.get("status") == "failed"
            and type(value.get("retry")) is int and value["retry"] == 0
            and type(value.get("totalErrorCount")) is int and 1 <= value["totalErrorCount"] <= 32
            and type(value.get("unknownErrorCount")) is int and value["unknownErrorCount"] == 0
            and value.get("assertionErrorsExact") is True
            and type(value.get("knownAssertionErrorIDs")) is list
            and len(value["knownAssertionErrorIDs"]) == value["totalErrorCount"])
