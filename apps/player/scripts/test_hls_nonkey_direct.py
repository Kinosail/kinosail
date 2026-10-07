"""Protect byte-range evidence that a successful watch cannot forge."""
import hashlib
import unittest
import hls_nonkey_direct


def response(status=200, span='', data=b'abcdef'):
    return {'status': status, 'contentRange': span, 'bytes': len(data),
            'sha256': hashlib.sha256(data).hexdigest()}


def network(rows=None):
    return {'unexpectedMediaRequests': 0, 'failedMediaResponses': 0,
            'directResponses': rows if rows is not None else [response()]}


class DirectDeliveryEvidence(unittest.TestCase):
    def test_whole_and_actual_partial_source_ranges_are_verified(self):
        for rows in [[response()], [response(206, 'bytes 0-5/6')],
                     [response(206, 'bytes 1-3/6', b'bcd')]]:
            self.assertTrue(hls_nonkey_direct.direct_delivery_matches(network(rows), b'abcdef'))

    def test_wrong_length_hash_and_status_fail(self):
        for key, bad in [('bytes', 5), ('bytes', True), ('status', 304),
                         ('sha256', '0' * 64), ('sha256', 'private-path')]:
            self.assertFalse(hls_nonkey_direct.direct_delivery_matches(
                network([response() | {key: bad}]), b'abcdef'))

    def test_ranges_cannot_escape_or_misidentify_source(self):
        for span in ['bytes 0-5/7', 'bytes -1-5/6', 'bytes 0-6/6',
                     'bytes 5-0/6', 'bytes */6', 'bytes 0-5/6\nsecret', '']:
            self.assertFalse(hls_nonkey_direct.direct_delivery_matches(
                network([response(206, span)]), b'abcdef'))
        self.assertFalse(hls_nonkey_direct.direct_delivery_matches(
            network([response(200, 'bytes 0-5/6')]), b'abcdef'))

    def test_failures_foreign_media_and_empty_evidence_fail(self):
        for value in [network([]), network([response()] * 33),
                      network() | {'unexpectedMediaRequests': 1},
                      network() | {'failedMediaResponses': 1}]:
            self.assertFalse(hls_nonkey_direct.direct_delivery_matches(value, b'abcdef'))

    def test_source_and_receipt_shapes_are_bounded(self):
        for source in [b'', b'x' * (8 * 1024 * 1024 + 1), 'abcdef']:
            self.assertFalse(hls_nonkey_direct.direct_delivery_matches(network(), source))
        for rows in ['bad', [None], [response() | {'url': 'private'}],
                     [response() | {'contentRange': 'x' * 81}]]:
            self.assertFalse(hls_nonkey_direct.direct_delivery_matches(network(rows), b'abcdef'))


if __name__ == '__main__':
    unittest.main()
