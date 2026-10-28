# Put birdROMs on your handheld

This is for handhelds running **muOS** (Anbernic RG40XX-V, RG35XX+ and similar). It takes about five
minutes and you do not need to type anything on the handheld. You need: your handheld, its SD card,
a computer that can read the SD card, and your phone.

## 1. Download the app

On your computer, open your friend group's birdROMs website and sign in. Click **Handheld** at the top.
Press **Download birdROMs.muxapp**. You now have one file called `birdROMs.muxapp`. (Do not unzip it.)

## 2. Copy it to the SD card

1. Turn the handheld off, take out the SD card that holds your games (the one named something like
   `ROMS-RG40`), and plug it into your computer.
2. Look for a folder named **ARCHIVE** at the very top of the card. If there is no such folder, make
   one (capital letters, exactly `ARCHIVE`).
3. Drag `birdROMs.muxapp` into the `ARCHIVE` folder.
4. Eject the card properly, put it back in the handheld, and turn it on.

If your handheld has two cards and you are not sure which one: either works. The games card is the
easy one to reach.

## 3. Install it on the handheld

1. On the muOS main menu, open **Applications**.
2. Open **Archive Manager**.
3. Find **birdROMs** in the list (it may have a short tag in front, like `[SD2-APP]`) and press **A**.
4. Wait until it says it is done, then press **B** a few times to go back to the main menu.

## 4. Open it and link it to your account

1. In **Applications**, open **birdROMs**.
2. The handheld shows a **QR code** and a short code. Scan the QR code with your phone camera.
3. Sign in on your phone if it asks, then press **Approve**.
4. The handheld moves on by itself. Pick a platform or playlist, choose games (A to tick, Y for all or
   none) and press **Start**. Games are saved in the right system folders automatically.

That is it. Next time just open **birdROMs** from Applications.

## Updating to a new version

When the person who runs the server tells you there is a new version:

1. Download `birdROMs.muxapp` again from the **Handheld** page.
2. Copy it into the `ARCHIVE` folder on the card and **replace** the old file.
3. Do step 3 above again (Applications, Archive Manager, birdROMs).

You stay linked to your account; you do not scan the QR code again.

## If something goes wrong

| What you see | What to do |
|---|---|
| No **ARCHIVE** folder or Archive Manager is empty | Check the folder is spelled `ARCHIVE` at the top of the card, not inside another folder, and the file ends in `.muxapp`. |
| The Handheld page says the app is not published | The server owner needs to publish it first. Ask them. |
| **birdROMs** is not in Applications after installing | Close and reopen Applications, or restart the handheld. |
| "No server configured" on the handheld | The file did not come from the website. Download it again from the Handheld page. |
| The QR code shows up again later | The owner removed your device from the server. Scan it again to re-link. |
