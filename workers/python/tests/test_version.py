from utils.logging import BUILD_VERSION, configure, get_build_version, get_logger


def test_build_fallback_when_file_missing():
    # When /app/version.txt does not exist on dev host, BUILD_VERSION falls back to "unknown" or file content
    assert isinstance(BUILD_VERSION, str)
    assert len(BUILD_VERSION) > 0
    assert get_build_version() == BUILD_VERSION


def test_all_loggers_include_build(capsys):
    configure()
    logger = get_logger("test.module")
    logger.info("test_event", foo="bar")
    captured = capsys.readouterr()
    assert '"build":' in captured.out
    assert '"event": "test_event"' in captured.out
