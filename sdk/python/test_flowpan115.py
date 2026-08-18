import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from flowpan115 import (
    DRIVER_COOKIE,
    Flowpan115Client,
    RetryPolicy,
    UploadInitRequest,
    UploadSession,
)


class _Handler(BaseHTTPRequestHandler):
    hits = 0
    last_payload = None

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        type(self).last_payload = json.loads(body.decode("utf-8"))
        type(self).hits += 1
        if self.path.endswith("/usage") and type(self).hits < 3:
            self.send_response(502)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"code":502,"msg":"temporary","data":{}}')
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(
            b'{"code":200,"data":{"driver":"cookie","result":'
            b'{"session_id":"s-1","name":"movie.mkv","size":10,'
            b'"target_cid":9,"parts":[{"part_number":1,"etag":"etag-1","size":10}]}}}'
        )

    def log_message(self, *_args):
        return


class Flowpan115ClientTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        _Handler.hits = 0
        _Handler.last_payload = None
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.base_url = f"http://127.0.0.1:{cls.server.server_port}"

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=2)

    def test_retry_and_driver(self):
        client = Flowpan115Client(
            self.base_url,
            "secret",
            retry_policy=RetryPolicy(
                max_attempts=3,
                initial_delay=0,
                max_delay=0,
            ),
        )
        result = client.usage(driver=DRIVER_COOKIE)
        self.assertEqual(result.driver, "cookie")
        self.assertEqual(_Handler.hits, 3)
        with self.assertRaises(ValueError):
            client.usage(driver="invalid")

    def test_action_validation(self):
        client = Flowpan115Client(self.base_url, "secret")
        with self.assertRaises(ValueError):
            client.call("usage?token=secret")

    def test_typed_upload_session(self):
        client = Flowpan115Client(self.base_url, "secret")
        result = client.upload_init_typed(
            UploadInitRequest(
                name="movie.mkv",
                size=10,
                sha1="A" * 40,
                target_cid=9,
            )
        )
        self.assertIsInstance(result.result, UploadSession)
        self.assertEqual(result.result.session_id, "s-1")
        self.assertEqual(_Handler.last_payload["name"], "movie.mkv")


if __name__ == "__main__":
    unittest.main()
