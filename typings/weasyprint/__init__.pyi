"""Type stubs for weasyprint."""

from typing import Any

class HTML:
    def __init__(
        self,
        guess: Any = None,
        filename: str | None = None,
        url: str | None = None,
        file_obj: Any = None,
        string: str | bytes | None = None,
        encoding: str | None = None,
        base_url: str | None = None,
        url_fetcher: Any = None,
        media_type: str = "print",
    ) -> None: ...
    def write_pdf(
        self,
        target: Any = None,
        zoom: float = 1,
        finisher: Any = None,
        font_config: Any = None,
        counter_style: Any = None,
        color_profiles: Any = None,
        **options: Any,
    ) -> bytes | None: ...

class CSS:
    def __init__(
        self,
        guess: Any = None,
        filename: str | None = None,
        url: str | None = None,
        file_obj: Any = None,
        string: str | bytes | None = None,
        encoding: str | None = None,
        base_url: str | None = None,
        url_fetcher: Any = None,
        media_type: str = "print",
        font_config: Any = None,
        counter_style: Any = None,
    ) -> None: ...
