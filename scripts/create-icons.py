#!/usr/bin/env python3
"""Generate ChessGrowth platform icons from assets/chessgrowth-icon.png.

Only for changing the artwork. Published source includes all generated icons.
Requires Pillow: python3 -m pip install Pillow
"""
from PIL import Image
from pathlib import Path
from collections import deque
from io import BytesIO
import struct
ROOT=Path(__file__).resolve().parent.parent
SRC=ROOT/'assets/chessgrowth-icon.png'
IMAGE=Image.open(SRC).convert('RGBA')
w,h=IMAGE.size
pixels=IMAGE.load()
# Clear only near-white exterior connected to the image border.
queue=deque();seen=bytearray(w*h)
for x in range(w):queue.append((x,0));queue.append((x,h-1))
for y in range(h):queue.append((0,y));queue.append((w-1,y))
while queue:
 x,y=queue.popleft();i=y*w+x
 if seen[i]:continue
 seen[i]=1
 r,g,b,a=pixels[x,y]
 if min(r,g,b)<238 or max(r,g,b)-min(r,g,b)>18:continue
 pixels[x,y]=(r,g,b,0)
 if x>0:queue.append((x-1,y))
 if x<w-1:queue.append((x+1,y))
 if y>0:queue.append((x,y-1))
 if y<h-1:queue.append((x,y+1))
# crop whitespace while retaining a small transparent gutter
bounds=IMAGE.getbbox()
if bounds:
 box=IMAGE.crop(bounds)
 side=max(box.size)
 target=Image.new('RGBA',(side+round(side*.045)*2,)*2,(0,0,0,0))
 target.alpha_composite(box,((target.width-box.width)//2,(target.height-box.height)//2))
 IMAGE=target.resize((1024,1024),Image.Resampling.LANCZOS)
IMAGE.save(SRC)
folder=ROOT/'packaging/windows_icon';folder.mkdir(parents=True,exist_ok=True)
for size in (16,24,32,48,64,128,256,512,1024):
 IMAGE.resize((size,size),Image.Resampling.LANCZOS).save(folder/f'icon_{size}.png')
# ICO with Windows-friendly embedded PNG resources
IMAGE.save(ROOT/'packaging/ChessGrowth.ico',format='ICO',sizes=[(s,s) for s in (16,24,32,48,64,128,256)])
# Modern PNG-backed ICNS, valid on Intel and Apple Silicon.
chunks=[]
for kind,size in [(b'icp4',16),(b'icp5',32),(b'icp6',64),(b'ic07',128),(b'ic08',256),(b'ic09',512),(b'ic10',1024),(b'ic11',32),(b'ic12',64),(b'ic13',256),(b'ic14',512)]:
 image=IMAGE.resize((size,size),Image.Resampling.LANCZOS)
 out=BytesIO();image.save(out,format='PNG');body=out.getvalue()
 chunks.append(kind+struct.pack('>I',len(body)+8)+body)
(ROOT/'packaging/ChessGrowth.icns').write_bytes(b'icns'+struct.pack('>I',8+sum(map(len,chunks)))+b''.join(chunks))
print('Created ChessGrowth assets, .ico and .icns')
