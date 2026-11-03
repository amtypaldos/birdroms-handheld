#!/usr/bin/env python3
"""Run rsvg-convert -w 224 -h 224 -o /tmp/bird-icon.png scripts/icon.svg first."""
from PIL import Image
image = Image.open('/tmp/bird-icon.png').convert('RGBA')
canvas = Image.new('RGB', (256, 256), (11, 13, 23))
canvas.paste(image, (16, 8), image)
canvas.save('assets/icon.jpg', quality=95, subsampling=0)
