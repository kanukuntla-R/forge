from typer.testing import CliRunner

from src.main import app

runner = CliRunner()


def test_hello_default() -> None:
    result = runner.invoke(app, ["hello"])
    assert result.exit_code == 0
    assert "Hello, World!" in result.output


def test_hello_with_name() -> None:
    result = runner.invoke(app, ["hello", "--name", "Ruthvik"])
    assert result.exit_code == 0
    assert "Hello, Ruthvik!" in result.output


def test_hello_shout() -> None:
    result = runner.invoke(app, ["hello", "--name", "world", "--shout"])
    assert result.exit_code == 0
    assert "HELLO, WORLD!" in result.output
