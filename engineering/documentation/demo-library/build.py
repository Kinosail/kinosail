#!/usr/bin/env python3
"""Build the fictional Kinosail demo library used for public screenshots."""
import argparse
import subprocess
from html import escape
from pathlib import Path

from catalog import FILMS, SHOWS
from scenes import backdrop, poster

HERE = Path(__file__).resolve().parent


def run(*command):
    subprocess.run(command, check=True)


def nfo(tag, fields):
    body = "".join(f"<{key}>{escape(str(value))}</{key}>" for key, values in fields.items()
                   for value in (values if isinstance(values, list) else [values]) if value)
    return f'<?xml version="1.0" encoding="UTF-8"?>\n<{tag}>{body}</{tag}>\n'


def subtitles(lines, minutes):
    cues, start = [], 4
    for index, line in enumerate(lines, 1):
        cues.append(f"{index}\n00:00:{start:02d},000 --> 00:00:{start + 3:02d},500\n{line}\n")
        start += 5
    return "\n".join(cues)


def jpeg(png, target, quality=3):
    run("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(png), "-q:v", str(quality), str(target))


def video(still, target, minutes, work):
    chunk = work / (target.stem + ".chunk.mp4")
    run("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-loop", "1", "-framerate", "24", "-i", str(still),
        "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "60", "-vf", "scale=1920:1080,format=yuv420p",
        "-c:v", "libx264", "-preset", "medium", "-tune", "stillimage", "-crf", "24", "-g", "1440",
        "-c:a", "aac", "-b:a", "48k", "-shortest", "-movflags", "+faststart", str(chunk))
    listing = work / (target.stem + ".txt")
    listing.write_text("".join(f"file '{chunk.name}'\n" for _ in range(minutes)))
    run("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", str(listing),
        "-c", "copy", "-movflags", "+faststart", str(target))
    chunk.unlink()
    listing.unlink()


def build(output, playwright, scale):
    art = output / "art"
    work = output / "work"
    media = output / "media"
    for directory in (art, work, media / "Movies", media / "Shows"):
        directory.mkdir(parents=True)
    slugs = {}
    for film in FILMS:
        slug = film["title"].lower().replace(" ", "-")
        slugs[film["title"]] = slug
        (art / f"{slug}-poster.svg").write_text(poster(film))
        (art / f"{slug}-backdrop.svg").write_text(backdrop(film))
    for show in SHOWS:
        slug = show["title"].lower().replace(" ", "-")
        (art / f"{slug}-poster.svg").write_text(poster(show, kind="Show"))
        (art / f"{slug}-backdrop.svg").write_text(backdrop(show))
        for season, episodes in enumerate(show["seasons"], 1):
            for number, _ in enumerate(episodes, 1):
                (art / f"{slug}-s{season:02d}e{number:02d}.svg").write_text(backdrop(show, 1280, 720, f"s{season}e{number}"))
    run("node", str(HERE / "render-art.cjs"), playwright, str(art))
    for film in FILMS:
        slug, name = slugs[film["title"]], f"{film['title']} ({film['year']})"
        folder = media / "Movies" / name
        folder.mkdir()
        jpeg(art / f"{slug}-poster.png", folder / f"{name}.jpg")
        jpeg(art / f"{slug}-backdrop.png", folder / f"{name}-fanart.jpg")
        (folder / f"{name}.nfo").write_text(nfo("movie", dict(title=film["title"], year=film["year"], plot=film["plot"],
            tagline=film["tagline"], genre=film["genres"], studio=film["studio"], mpaa=film["rating"], director=film["director"])))
        (folder / f"{name}.en.srt").write_text(subtitles(film["lines"], film["minutes"]))
        video(art / f"{slug}-backdrop.png", folder / f"{name}.mp4", max(1, film["minutes"] // scale), work)
    for show in SHOWS:
        slug = show["title"].lower().replace(" ", "-")
        folder = media / "Shows" / show["title"]
        folder.mkdir()
        jpeg(art / f"{slug}-poster.png", folder / "poster.jpg")
        jpeg(art / f"{slug}-backdrop.png", folder / "fanart.jpg")
        for season, episodes in enumerate(show["seasons"], 1):
            season_folder = folder / f"Season {season:02d}"
            season_folder.mkdir()
            for number, title in enumerate(episodes, 1):
                base = season_folder / f"{show['title']} S{season:02d}E{number:02d} {title}"
                still = art / f"{slug}-s{season:02d}e{number:02d}.png"
                jpeg(still, base.with_name(base.name + ".jpg"))
                plot = f"{title}: an episode of the fictional {show['genres'][0].lower()} series {show['title']}."
                base.with_name(base.name + ".nfo").write_text(nfo("episodedetails", dict(title=title, plot=plot,
                    aired=f"{show['year'] + season - 1}-0{min(number, 9)}-1{number % 10}")))
                video(still, base.with_name(base.name + ".mp4"), max(1, show["minutes"] // scale), work)
    work.rmdir()
    print(f"Demo library ready at {media}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--playwright", required=True, help="Path to the @playwright/test module")
    parser.add_argument("--scale", type=int, default=1, help="Divide runtimes for quick previews")
    args = parser.parse_args()
    if args.output.exists():
        parser.error("output must not exist")
    build(args.output.resolve(), args.playwright, args.scale)
