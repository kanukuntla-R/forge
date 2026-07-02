import typer


def hello(
    name: str = typer.Option("World", "--name", "-n", help="Person to greet"),
    shout: bool = typer.Option(False, "--shout", help="Shout the greeting"),
) -> None:
    """Say hello to someone."""
    greeting = f"Hello, {name}!"
    if shout:
        greeting = greeting.upper()
    typer.echo(greeting)
