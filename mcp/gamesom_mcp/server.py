"""MCP server exposing the gamesom library database to Claude."""

import json
import os
import sqlite3
from pathlib import Path
from typing import Any

from mcp.server.fastmcp import FastMCP

mcp = FastMCP("gamesom")


def db_path() -> Path:
    xdg = os.environ.get("XDG_DATA_HOME")
    if xdg:
        return Path(xdg) / "gamesom" / "gamesom.db"
    return Path.home() / ".local" / "share" / "gamesom" / "gamesom.db"


def get_conn() -> sqlite3.Connection:
    conn = sqlite3.connect(db_path())
    conn.row_factory = sqlite3.Row
    return conn


@mcp.tool()
def list_games(
    installed_only: bool = False,
    source: str | None = None,
    has_profile: bool | None = None,
    limit: int = 50,
    offset: int = 0,
) -> list[dict[str, Any]]:
    """List games from the library with optional filters.

    Args:
        installed_only: Only return games with at least one installed library entry.
        source: Filter by import source (steam, epic, gog, amazon, itchio).
        has_profile: True = only games with a sommelier_profile; False = only games without.
        limit: Max results (default 50).
        offset: Pagination offset.
    """
    wheres = []
    params: list[Any] = []

    if installed_only:
        wheres.append("EXISTS (SELECT 1 FROM library_entries le WHERE le.game_id = g.id AND le.installed = 1)")

    if source:
        wheres.append("EXISTS (SELECT 1 FROM library_entries le WHERE le.game_id = g.id AND le.source = ?)")
        params.append(source)

    if has_profile is True:
        wheres.append("EXISTS (SELECT 1 FROM sommelier_profile sp WHERE sp.game_id = g.id)")
    elif has_profile is False:
        wheres.append("NOT EXISTS (SELECT 1 FROM sommelier_profile sp WHERE sp.game_id = g.id)")

    where_clause = ("WHERE " + " AND ".join(wheres)) if wheres else ""
    params += [limit, offset]

    query = f"""
        SELECT
            g.id, g.canonical_title, g.genres, g.themes, g.summary,
            g.first_release_date,
            MAX(le.installed) AS installed,
            SUM(le.playtime_minutes) AS playtime_minutes,
            MAX(le.last_played_at) AS last_played_at,
            GROUP_CONCAT(DISTINCT le.source) AS sources,
            sp.energy_required, sp.friction_level, sp.session_length_fit,
            sp.narrative_memory_load, sp.complexity_level,
            sp.mood_tags, sp.avoid_when, sp.best_when, sp.user_notes
        FROM games g
        LEFT JOIN library_entries le ON le.game_id = g.id
        LEFT JOIN sommelier_profile sp ON sp.game_id = g.id
        {where_clause}
        GROUP BY g.id
        ORDER BY g.canonical_title
        LIMIT ? OFFSET ?
    """

    with get_conn() as conn:
        rows = conn.execute(query, params).fetchall()
    return [dict(r) for r in rows]


@mcp.tool()
def search_games(query: str, limit: int = 20) -> list[dict[str, Any]]:
    """Search games by title (case-insensitive substring match).

    Args:
        query: Title substring to search for.
        limit: Max results.
    """
    sql = """
        SELECT
            g.id, g.canonical_title, g.genres, g.summary,
            MAX(le.installed) AS installed,
            SUM(le.playtime_minutes) AS playtime_minutes,
            GROUP_CONCAT(DISTINCT le.source) AS sources,
            sp.energy_required, sp.friction_level, sp.session_length_fit,
            sp.mood_tags, sp.avoid_when, sp.best_when
        FROM games g
        LEFT JOIN library_entries le ON le.game_id = g.id
        LEFT JOIN sommelier_profile sp ON sp.game_id = g.id
        WHERE g.canonical_title LIKE ?
        GROUP BY g.id
        ORDER BY g.canonical_title
        LIMIT ?
    """
    with get_conn() as conn:
        rows = conn.execute(sql, [f"%{query}%", limit]).fetchall()
    return [dict(r) for r in rows]


@mcp.tool()
def get_game(game_id: int) -> dict[str, Any] | None:
    """Get full details for a single game including all library entries and sommelier profile.

    Args:
        game_id: The game's database ID.
    """
    with get_conn() as conn:
        game = conn.execute("SELECT * FROM games WHERE id = ?", [game_id]).fetchone()
        if not game:
            return None
        entries = conn.execute(
            "SELECT * FROM library_entries WHERE game_id = ?", [game_id]
        ).fetchall()
        profile = conn.execute(
            "SELECT * FROM sommelier_profile WHERE game_id = ?", [game_id]
        ).fetchone()

    return {
        "game": dict(game),
        "library_entries": [dict(e) for e in entries],
        "sommelier_profile": dict(profile) if profile else None,
    }


@mcp.tool()
def upsert_profile(
    game_id: int,
    energy_required: str | None = None,
    friction_level: str | None = None,
    session_length_fit: str | None = None,
    narrative_memory_load: str | None = None,
    complexity_level: str | None = None,
    mood_tags: str | None = None,
    avoid_when: str | None = None,
    best_when: str | None = None,
    user_notes: str | None = None,
) -> str:
    """Write or update the sommelier profile for a game.

    Use this during a recommendation conversation to persist trait inferences
    so they survive across sessions.

    Args:
        game_id: The game's database ID.
        energy_required: low | medium | high
        friction_level: low | medium | high
        session_length_fit: 5-15min | 15-45min | 45min-2h | 2h+ | variable
        narrative_memory_load: none | low | medium | high
        complexity_level: minimal | low | medium | high | very high
        mood_tags: Comma-separated mood words (e.g. "relaxing, cozy, dark").
        avoid_when: Short phrase (e.g. "tired, foggy, low time").
        best_when: Short phrase (e.g. "want quick wins, cozy evening").
        user_notes: Free-form notes from the conversation.
    """
    with get_conn() as conn:
        conn.execute(
            """
            INSERT INTO sommelier_profile
                (game_id, energy_required, friction_level, session_length_fit,
                 narrative_memory_load, complexity_level, mood_tags, avoid_when,
                 best_when, user_notes)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(game_id) DO UPDATE SET
                energy_required     = COALESCE(excluded.energy_required, energy_required),
                friction_level      = COALESCE(excluded.friction_level, friction_level),
                session_length_fit  = COALESCE(excluded.session_length_fit, session_length_fit),
                narrative_memory_load = COALESCE(excluded.narrative_memory_load, narrative_memory_load),
                complexity_level    = COALESCE(excluded.complexity_level, complexity_level),
                mood_tags           = COALESCE(excluded.mood_tags, mood_tags),
                avoid_when          = COALESCE(excluded.avoid_when, avoid_when),
                best_when           = COALESCE(excluded.best_when, best_when),
                user_notes          = COALESCE(excluded.user_notes, user_notes)
            """,
            [
                game_id, energy_required, friction_level, session_length_fit,
                narrative_memory_load, complexity_level, mood_tags,
                avoid_when, best_when, user_notes,
            ],
        )
    return f"Profile saved for game_id={game_id}"


@mcp.resource("gamesom://library/summary")
def library_summary() -> str:
    """A summary of the full game library — counts by source, installed state, enrichment status."""
    with get_conn() as conn:
        total = conn.execute("SELECT COUNT(*) FROM games").fetchone()[0]
        installed = conn.execute(
            "SELECT COUNT(DISTINCT game_id) FROM library_entries WHERE installed = 1"
        ).fetchone()[0]
        with_profile = conn.execute("SELECT COUNT(*) FROM sommelier_profile").fetchone()[0]
        sources = conn.execute(
            "SELECT source, COUNT(*) as n FROM library_entries GROUP BY source ORDER BY n DESC"
        ).fetchall()

    lines = [
        f"Total games: {total}",
        f"Installed: {installed}",
        f"With sommelier profile: {with_profile}",
        "",
        "By source:",
    ] + [f"  {r['source']}: {r['n']}" for r in sources]

    return "\n".join(lines)


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()
