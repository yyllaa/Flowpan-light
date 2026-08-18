"""Small standard-library client for Flowpan's external 115 storage API."""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass, field
from typing import Any, Callable, Mapping, Optional
from urllib.parse import quote


__version__ = "0.2.0"

DRIVER_AUTO = "auto"
DRIVER_OPEN = "open"
DRIVER_COOKIE = "cookie"
_VALID_DRIVERS = frozenset({DRIVER_AUTO, DRIVER_OPEN, DRIVER_COOKIE})
_DEFAULT_MAX_RESPONSE_BODY = 4 * 1024 * 1024
_RETRYABLE_ACTIONS = frozenset(
    {
        "dir/list",
        "dir/list-page",
        "search",
        "item",
        "usage",
        "download-url",
        "move/check-conflict",
        "upload/list-parts",
        "upload/part-url",
    }
)


@dataclass
class StorageResponse:
    driver: str
    result: Any
    data: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class RetryPolicy:
    """Transient retry policy for safe API actions."""

    max_attempts: int = 3
    initial_delay: float = 0.2
    max_delay: float = 2.0
    retryable: Optional[
        Callable[[str, int, int, Optional[BaseException]], bool]
    ] = None

    def should_retry(
        self,
        action: str,
        attempt: int,
        status_code: int,
        error: Optional[BaseException],
    ) -> bool:
        if attempt >= max(1, self.max_attempts):
            return False
        if self.retryable is not None:
            return self.retryable(action, attempt, status_code, error)
        if action not in _RETRYABLE_ACTIONS:
            return False
        if status_code in {408, 425, 429, 500, 502, 503, 504}:
            return True
        return isinstance(error, urllib.error.URLError)

    def delay(self, attempt: int, retry_after: str = "") -> float:
        parsed = _parse_retry_after(retry_after)
        if parsed > 0:
            return min(parsed, max(0.0, self.max_delay))
        delay = max(0.0, self.initial_delay)
        for _ in range(1, attempt):
            delay = min(max(0.0, self.max_delay), delay * 2)
        return min(delay, max(0.0, self.max_delay))


@dataclass
class UploadInitRequest:
    name: str
    size: int
    sha1: str
    target_cid: int
    preid: str = ""
    sign_key: str = ""
    sign_val: str = ""
    pickcode: str = ""

    def to_payload(self) -> dict[str, Any]:
        return _compact(asdict(self))


@dataclass
class UploadPart:
    part_number: int
    etag: str
    size: int = 0

    @classmethod
    def from_mapping(cls, value: Mapping[str, Any]) -> "UploadPart":
        return cls(
            part_number=int(value.get("part_number") or 0),
            etag=str(value.get("etag") or ""),
            size=int(value.get("size") or 0),
        )


@dataclass
class UploadSession:
    session_id: str = ""
    name: str = ""
    size: int = 0
    sha1: str = ""
    preid: str = ""
    target_cid: int = 0
    target: str = ""
    reused: bool = False
    requires_sign: bool = False
    status: int = 0
    code: int = 0
    sign_key: str = ""
    sign_check: str = ""
    range_start: int = 0
    range_end: int = 0
    file_id: int = 0
    pickcode: str = ""
    bucket: str = ""
    object: str = ""
    upload_url: str = ""
    callback: dict[str, str] = field(default_factory=dict)
    upload_id: str = ""
    part_size: int = 0
    uploaded: int = 0
    parts: list[UploadPart] = field(default_factory=list)
    response: dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_mapping(cls, value: Any) -> "UploadSession":
        if not isinstance(value, Mapping):
            raise Flowpan115Error("Flowpan upload result is not an object")
        parts = [
            UploadPart.from_mapping(part)
            for part in value.get("parts") or []
            if isinstance(part, Mapping)
        ]
        raw_callback = value.get("callback")
        callback = (
            {
                str(key): str(item)
                for key, item in raw_callback.items()
            }
            if isinstance(raw_callback, Mapping)
            else {}
        )
        raw_response = value.get("response")
        response = dict(raw_response) if isinstance(raw_response, Mapping) else {}
        return cls(
            session_id=str(value.get("session_id") or ""),
            name=str(value.get("name") or ""),
            size=int(value.get("size") or 0),
            sha1=str(value.get("sha1") or ""),
            preid=str(value.get("preid") or ""),
            target_cid=int(value.get("target_cid") or 0),
            target=str(value.get("target") or ""),
            reused=bool(value.get("reused")),
            requires_sign=bool(value.get("requires_sign")),
            status=int(value.get("status") or 0),
            code=int(value.get("code") or 0),
            sign_key=str(value.get("sign_key") or ""),
            sign_check=str(value.get("sign_check") or ""),
            range_start=int(value.get("range_start") or 0),
            range_end=int(value.get("range_end") or 0),
            file_id=int(value.get("file_id") or 0),
            pickcode=str(value.get("pickcode") or ""),
            bucket=str(value.get("bucket") or ""),
            object=str(value.get("object") or ""),
            upload_url=str(value.get("upload_url") or ""),
            callback=callback,
            upload_id=str(value.get("upload_id") or ""),
            part_size=int(value.get("part_size") or 0),
            uploaded=int(value.get("uploaded") or 0),
            parts=parts,
            response=response,
        )

    def to_payload(self) -> dict[str, Any]:
        return asdict(self)


@dataclass
class UploadPartURL:
    method: str
    url: str
    headers: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_mapping(cls, value: Any) -> "UploadPartURL":
        if not isinstance(value, Mapping):
            raise Flowpan115Error("Flowpan upload part URL result is not an object")
        return cls(
            method=str(value.get("method") or ""),
            url=str(value.get("url") or ""),
            headers=(
                {
                    str(key): str(item)
                    for key, item in raw_headers.items()
                }
                if isinstance(
                    raw_headers := value.get("headers"),
                    Mapping,
                )
                else {}
            ),
        )


class Flowpan115Error(RuntimeError):
    """Raised for HTTP errors or non-200 Flowpan response codes."""

    def __init__(
        self,
        message: str,
        *,
        status_code: int = 0,
        code: int = 0,
        body: str = "",
    ) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.code = code
        self.body = body


class Flowpan115Client:
    """Client for /api/v1/storage/115/{driver}/{action}."""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        default_driver: str = DRIVER_AUTO,
        timeout: float = 30.0,
        retry_policy: Optional[RetryPolicy] = None,
        user_agent: str = "",
        max_response_body: int = _DEFAULT_MAX_RESPONSE_BODY,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token.strip()
        self.default_driver = self._normalize_driver(default_driver)
        self.timeout = timeout
        self.retry_policy = retry_policy or RetryPolicy()
        self.user_agent = user_agent.strip() or f"flowpan115-python/{__version__}"
        self.max_response_body = max(1, max_response_body)

    def set_default_driver(self, driver: str) -> None:
        """Change the driver used when a call does not pass driver=."""
        self.default_driver = self._normalize_driver(driver)

    def with_driver(self, driver: str) -> "Flowpan115Client":
        """Return a new client sharing base URL/token but using another driver."""
        return Flowpan115Client(
            self.base_url,
            self.token,
            default_driver=self._normalize_driver(driver),
            timeout=self.timeout,
            retry_policy=self.retry_policy,
            user_agent=self.user_agent,
            max_response_body=self.max_response_body,
        )

    def call(
        self,
        action: str,
        payload: Any = None,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        selected_driver = self._normalize_driver(driver)
        action = _validate_action(action)
        if not self.base_url:
            raise ValueError("Flowpan base URL is empty")
        if not self.token:
            raise ValueError("Flowpan external API token is empty")

        body = json.dumps({} if payload is None else payload, ensure_ascii=False).encode("utf-8")
        endpoint = (
            f"{self.base_url}/api/v1/storage/115/"
            f"{quote(selected_driver, safe='')}/{action}"
        )
        policy = self.retry_policy
        for attempt in range(1, max(1, policy.max_attempts) + 1):
            request = urllib.request.Request(
                endpoint,
                data=body,
                method="POST",
                headers={
                    "Accept": "application/json",
                    "Authorization": f"Bearer {self.token}",
                    "Content-Type": "application/json; charset=utf-8",
                    "User-Agent": self.user_agent,
                },
            )
            try:
                with urllib.request.urlopen(request, timeout=self.timeout) as response:
                    status_code = response.status
                    response_bytes = response.read(self.max_response_body + 1)
                    retry_after = response.headers.get("Retry-After", "")
            except urllib.error.HTTPError as exc:
                response_bytes = exc.read(self.max_response_body + 1)
                if len(response_bytes) > self.max_response_body:
                    raise Flowpan115Error(
                        f"Flowpan response exceeds {self.max_response_body} bytes",
                        status_code=exc.code,
                    ) from exc
                response_body = response_bytes.decode("utf-8", errors="replace")
                try:
                    envelope = self._decode_envelope(response_body)
                except Flowpan115Error:
                    envelope = {}
                error = self._api_error(
                    status_code=exc.code,
                    envelope=envelope,
                    body=response_body,
                )
                if policy.should_retry(action, attempt, exc.code, error):
                    time.sleep(policy.delay(attempt, exc.headers.get("Retry-After", "")))
                    continue
                raise error from exc
            except urllib.error.URLError as exc:
                error = Flowpan115Error(f"Flowpan request failed: {exc.reason}")
                if policy.should_retry(action, attempt, 0, exc):
                    time.sleep(policy.delay(attempt))
                    continue
                raise error from exc

            if len(response_bytes) > self.max_response_body:
                raise Flowpan115Error(
                    f"Flowpan response exceeds {self.max_response_body} bytes",
                    status_code=status_code,
                )
            response_body = response_bytes.decode("utf-8", errors="replace")
            envelope = self._decode_envelope(response_body)
            if status_code < 200 or status_code >= 300:
                error = self._api_error(
                    status_code=status_code,
                    envelope=envelope,
                    body=response_body,
                )
                if policy.should_retry(action, attempt, status_code, error):
                    time.sleep(policy.delay(attempt, retry_after))
                    continue
                raise error
            if envelope.get("code") != 200:
                error = self._api_error(
                    status_code=status_code,
                    envelope=envelope,
                    body=response_body,
                )
                if policy.should_retry(action, attempt, status_code, error):
                    time.sleep(policy.delay(attempt, retry_after))
                    continue
                raise error

            data = envelope.get("data") or {}
            if not isinstance(data, dict):
                raise Flowpan115Error("Flowpan response data is not an object")
            return StorageResponse(
                driver=str(data.get("driver") or selected_driver),
                result=data.get("result"),
                data=data,
            )

    def usage(self, *, driver: Optional[str] = None) -> StorageResponse:
        return self.call("usage", {}, driver=driver)

    def ensure_dir(
        self,
        path: str,
        *,
        cid: int = 0,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("dir/ensure", {"cid": cid, "path": path}, driver=driver)

    def list_dir(
        self,
        *,
        path: str = "",
        cid: int = 0,
        type: str = "",
        storage: str = "",
        driver: Optional[str] = None,
    ) -> StorageResponse:
        payload = _compact(
            {"path": path, "cid": cid, "type": type, "storage": storage}
        )
        return self.call("dir/list", payload, driver=driver)

    def list_page(
        self,
        *,
        cid: int = 0,
        offset: int = 0,
        limit: int = 100,
        driver: Optional[str] = None,
        **options: Any,
    ) -> StorageResponse:
        payload = _compact({"cid": cid, "offset": offset, "limit": limit, **options})
        return self.call("dir/list-page", payload, driver=driver)

    def search(
        self,
        keyword: str,
        *,
        cid: int = 0,
        offset: int = 0,
        limit: int = 100,
        driver: Optional[str] = None,
        **options: Any,
    ) -> StorageResponse:
        payload = _compact(
            {
                "keyword": keyword,
                "cid": cid,
                "offset": offset,
                "limit": limit,
                **options,
            }
        )
        return self.call("search", payload, driver=driver)

    def item(
        self,
        *,
        path: str = "",
        cid: int = 0,
        type: str = "",
        storage: str = "",
        driver: Optional[str] = None,
    ) -> StorageResponse:
        payload = _compact(
            {"path": path, "cid": cid, "type": type, "storage": storage}
        )
        return self.call("item", payload, driver=driver)

    def rename(
        self,
        file_id: int,
        name: str,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "rename",
            {"file_id": file_id, "name": name},
            driver=driver,
        )

    def move(
        self,
        ids: list[int],
        parent_id: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "move",
            {"ids": ids, "parent_id": parent_id},
            driver=driver,
        )

    def copy(
        self,
        ids: list[int],
        parent_id: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "copy",
            {"ids": ids, "parent_id": parent_id},
            driver=driver,
        )

    def paste(
        self,
        action: str,
        ids: list[int],
        parent_id: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "paste",
            {"action": action, "ids": ids, "parent_id": parent_id},
            driver=driver,
        )

    def delete(
        self,
        ids: list[int],
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("delete", {"ids": ids}, driver=driver)

    def star(
        self,
        ids: list[int],
        starred: bool,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "star",
            {"ids": ids, "starred": starred},
            driver=driver,
        )

    def move_check_conflict(
        self,
        ids: list[int],
        parent_id: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "move/check-conflict",
            {"ids": ids, "parent_id": parent_id},
            driver=driver,
        )

    def download_url(
        self,
        pickcode: str,
        *,
        user_agent: str = "",
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "download-url",
            _compact({"pickcode": pickcode, "user_agent": user_agent}),
            driver=driver,
        )

    def upload_file(
        self,
        local_path: str,
        target_cid: int,
        *,
        name: str = "",
        part_size: int = 0,
    ) -> StorageResponse:
        return self.call(
            "upload/file",
            _compact(
                {
                    "local_path": local_path,
                    "name": name,
                    "target_cid": target_cid,
                    "part_size": part_size,
                }
            ),
            driver=DRIVER_OPEN,
        )

    def upload_init(
        self,
        payload: Any,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("upload/init", payload, driver=driver)

    def upload_start(
        self,
        session: Any,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("upload/start", {"session": session}, driver=driver)

    def upload_resume(
        self,
        session: Any,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("upload/resume", {"session": session}, driver=driver)

    def upload_list_parts(
        self,
        session: Any,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call("upload/list-parts", {"session": session}, driver=driver)

    def upload_part_url(
        self,
        session: Any,
        part_number: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "upload/part-url",
            {"session": session, "part_number": part_number},
            driver=driver,
        )

    def upload_complete(
        self,
        session: Any,
        parts: Any,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        return self.call(
            "upload/complete",
            {"session": session, "parts": parts},
            driver=driver,
        )

    def upload_init_typed(
        self,
        request: UploadInitRequest,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_init(request.to_payload(), driver=driver)
        return StorageResponse(
            driver=response.driver,
            result=UploadSession.from_mapping(response.result),
            data=response.data,
        )

    def upload_start_typed(
        self,
        session: UploadSession,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_start(session.to_payload(), driver=driver)
        return StorageResponse(
            driver=response.driver,
            result=UploadSession.from_mapping(response.result),
            data=response.data,
        )

    def upload_resume_typed(
        self,
        session: UploadSession,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_resume(session.to_payload(), driver=driver)
        return StorageResponse(
            driver=response.driver,
            result=UploadSession.from_mapping(response.result),
            data=response.data,
        )

    def upload_list_parts_typed(
        self,
        session: UploadSession,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_list_parts(session.to_payload(), driver=driver)
        return StorageResponse(
            driver=response.driver,
            result=UploadSession.from_mapping(response.result),
            data=response.data,
        )

    def upload_part_url_typed(
        self,
        session: UploadSession,
        part_number: int,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_part_url(
            session.to_payload(),
            part_number,
            driver=driver,
        )
        return StorageResponse(
            driver=response.driver,
            result=UploadPartURL.from_mapping(response.result),
            data=response.data,
        )

    def upload_complete_typed(
        self,
        session: UploadSession,
        parts: list[UploadPart],
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        response = self.upload_complete(
            session.to_payload(),
            [asdict(part) for part in parts],
            driver=driver,
        )
        return StorageResponse(
            driver=response.driver,
            result=UploadSession.from_mapping(response.result),
            data=response.data,
        )

    def _normalize_driver(self, driver: Optional[str]) -> str:
        fallback = getattr(self, "default_driver", DRIVER_AUTO)
        value = (driver if driver is not None else fallback).strip().lower()
        if value not in _VALID_DRIVERS:
            raise ValueError(
                f"Flowpan driver {value!r} is invalid; want auto, open, or cookie"
            )
        return value

    @staticmethod
    def _decode_envelope(body: str) -> dict[str, Any]:
        try:
            value = json.loads(body)
        except json.JSONDecodeError as exc:
            raise Flowpan115Error(f"Flowpan response is not JSON: {exc}") from exc
        if not isinstance(value, dict):
            raise Flowpan115Error("Flowpan response is not an object")
        return value

    @staticmethod
    def _api_error(
        *,
        status_code: int,
        envelope: dict[str, Any],
        body: str,
    ) -> Flowpan115Error:
        code = int(envelope.get("code") or 0)
        message = str(
            envelope.get("msg")
            or envelope.get("message")
            or envelope.get("error")
            or f"HTTP {status_code}"
        )
        return Flowpan115Error(
            f"Flowpan API error: status={status_code} code={code} msg={message}",
            status_code=status_code,
            code=code,
            body=body.strip(),
        )


def _compact(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        key: value
        for key, value in payload.items()
        if value is not None
        and value != ""
        and (isinstance(value, bool) or value != 0)
    }


def _parse_retry_after(value: str) -> float:
    try:
        seconds = float(value.strip())
    except (AttributeError, ValueError):
        return 0.0
    return max(0.0, seconds)


def _validate_action(action: str) -> str:
    value = action.strip().strip("/")
    if not value:
        raise ValueError("Flowpan storage action is empty")
    if any(char in value for char in "?#\\") or "//" in value:
        raise ValueError(f"Flowpan storage action is invalid: {value!r}")
    for part in value.split("/"):
        if not part or part in {".", ".."}:
            raise ValueError(f"Flowpan storage action is invalid: {value!r}")
        if not all(char.isalnum() or char in "-_" for char in part):
            raise ValueError(f"Flowpan storage action is invalid: {value!r}")
    return value


__all__ = [
    "DRIVER_AUTO",
    "DRIVER_OPEN",
    "DRIVER_COOKIE",
    "RetryPolicy",
    "UploadInitRequest",
    "UploadPart",
    "UploadPartURL",
    "UploadSession",
    "Flowpan115Client",
    "Flowpan115Error",
    "StorageResponse",
    "__version__",
]
