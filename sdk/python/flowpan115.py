"""Small standard-library client for Flowpan's external 115 storage API."""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any, Optional
from urllib.parse import quote


DRIVER_AUTO = "auto"
DRIVER_OPEN = "open"
DRIVER_COOKIE = "cookie"


@dataclass
class StorageResponse:
    driver: str
    result: Any


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
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token.strip()
        self.default_driver = default_driver.strip().lower() or DRIVER_AUTO
        self.timeout = timeout

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
        )

    def call(
        self,
        action: str,
        payload: Any = None,
        *,
        driver: Optional[str] = None,
    ) -> StorageResponse:
        selected_driver = self._normalize_driver(driver)
        action = action.strip("/")
        if not self.base_url:
            raise ValueError("Flowpan base URL is empty")
        if not self.token:
            raise ValueError("Flowpan external API token is empty")
        if not action:
            raise ValueError("Flowpan storage action is empty")

        body = json.dumps({} if payload is None else payload, ensure_ascii=False).encode("utf-8")
        endpoint = (
            f"{self.base_url}/api/v1/storage/115/"
            f"{quote(selected_driver, safe='')}/{action}"
        )
        request = urllib.request.Request(
            endpoint,
            data=body,
            method="POST",
            headers={
                "Accept": "application/json",
                "Authorization": f"Bearer {self.token}",
                "Content-Type": "application/json; charset=utf-8",
            },
        )

        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                status_code = response.status
                response_body = response.read().decode("utf-8", errors="replace")
        except urllib.error.HTTPError as exc:
            response_body = exc.read().decode("utf-8", errors="replace")
            envelope = self._decode_envelope(response_body)
            raise self._api_error(
                status_code=exc.code,
                envelope=envelope,
                body=response_body,
            ) from exc
        except urllib.error.URLError as exc:
            raise Flowpan115Error(f"Flowpan request failed: {exc.reason}") from exc

        envelope = self._decode_envelope(response_body)
        if status_code < 200 or status_code >= 300:
            raise self._api_error(
                status_code=status_code,
                envelope=envelope,
                body=response_body,
            )
        if envelope.get("code") != 200:
            raise self._api_error(
                status_code=status_code,
                envelope=envelope,
                body=response_body,
            )

        data = envelope.get("data") or {}
        return StorageResponse(
            driver=str(data.get("driver") or selected_driver),
            result=data.get("result"),
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

    def _normalize_driver(self, driver: Optional[str]) -> str:
        value = (driver or self.default_driver).strip().lower()
        if value not in {DRIVER_AUTO, DRIVER_OPEN, DRIVER_COOKIE}:
            return DRIVER_AUTO
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


__all__ = [
    "DRIVER_AUTO",
    "DRIVER_OPEN",
    "DRIVER_COOKIE",
    "Flowpan115Client",
    "Flowpan115Error",
    "StorageResponse",
]
