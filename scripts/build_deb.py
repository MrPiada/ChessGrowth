#!/usr/bin/env python3
"""Create a Debian chessgrowth .deb from a Go binary using only Python stdlib.

Usable on macOS, Linux, and Windows. No dpkg-deb required to build.
"""
import argparse
import gzip
import io
import os
import tarfile
from pathlib import Path


def add_file(tar, name, data, mode=0o644):
    info = tarfile.TarInfo('./' + name.lstrip('/'))
    info.size = len(data)
    info.mode = mode
    info.mtime = 0
    info.uid = info.gid = 0
    tar.addfile(info, io.BytesIO(data))


def make_tar(files):
    buffer = io.BytesIO()
    with gzip.GzipFile(fileobj=buffer, mode='wb', mtime=0) as gz:
        with tarfile.open(fileobj=gz, mode='w|') as tar:
            for name, data, perms in files:
                add_file(tar, name, data, perms)
    return buffer.getvalue()


def ar_entry(name, data):
    # ar archives use even-byte alignment and standard 60-byte member headers.
    fields = [name+'/', '0', '0', '0', '100644', str(len(data))]
    sizes = [16, 12, 6, 6, 8, 10]
    head = ''.join(v.ljust(w) for v,w in zip(fields,sizes)).encode('ascii') + b'`\n'
    return head + data + (b'\n' if len(data)%2 else b'')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--arch', choices=['amd64','arm64'], required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    version = args.version.removeprefix('v').split('-')[0]
    exe = args.binary.read_bytes()
    icon = (root/'packaging/windows_icon/icon_256.png').read_bytes()
    desktop = (root/'packaging/ChessGrowth.desktop').read_bytes().replace(b'Exec=ChessGrowth',b'Exec=/usr/bin/chessgrowth')
    license_data=(root/'LICENSE').read_bytes()
    control=(f'''Package: chessgrowth
Version: {version}
Section: games
Priority: optional
Architecture: {args.arch}
Maintainer: Federico Versari <noreply@github.com>
Installed-Size: {(len(exe)+len(icon)+len(desktop)+len(license_data))//1024+1}
Description: Native chess opening repertoire generator and editor
 Build a chess repertoire using Lichess opening statistics and optional
 engine analysis; prune branches and export annotated PGN files.
''').encode()
    ctl=make_tar([('control',control,0o644)])
    data=make_tar([
        ('usr/bin/chessgrowth',exe,0o755),
        ('usr/share/applications/chessgrowth.desktop',desktop,0o644),
        ('usr/share/icons/hicolor/256x256/apps/chessgrowth.png',icon,0o644),
        ('usr/share/doc/chessgrowth/copyright',license_data,0o644),
    ])
    args.output.write_bytes(b'!<arch>\n'+ar_entry('debian-binary',b'2.0\n')+ar_entry('control.tar.gz',ctl)+ar_entry('data.tar.gz',data))
    print(f'Debian package: {args.output}')

if __name__=='__main__': main()
