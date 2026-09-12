(() => {
  const ENTRIES = [
    {
      date: "2026-09-12",
      items: {
        de: [
          "Der Termin-Chat läuft schon in der Abstimmung, nicht erst nach Annehmen.",
          "Planer und Verwaltung setzen eine Stimme für jemand anderen mit vier Symbolen. So gesetzte Stimmen sind blau.",
          "Termine starten zugeklappt. Ein Zugeklappt gilt für alle. Zu sehen bleiben Titel, Zeit, Art, das Stimmen-Gesicht, die Symbole, Ja/Vielleicht/Nein und das Menü.",
          "Annehmen, Absagen und Löschen eines Termins fragen zuerst nach, weil sich das nicht rückgängig machen lässt.",
          "Über der Alarmsirene auf der Titelliste stehen die Namen, die Oh Schreck getippt haben.",
          "Löschen eines Termins fragt zuerst: Willst du diesen Termin löschen?",
          "Oben am Termin liegen die Symbole, das Menü und für Planer Annehmen, Absagen und Löschen als farbige Gruppe. Dieselben Farben hat das Menü.",
          "Die Teilnahme am Termin zeigt wieder nur das aktuelle Gesicht.",
          "Oh Schreck auf der Titelliste ist eine rote Alarmsirene (🚨).",
          "Annehmen, Absagen und Löschen am eigenen Termin liegen im Menü, getrennt von Ja, Vielleicht und Nein.",
          "Am Termin ersetzen Symbole die Knöpfe Kommentare, Titel, Termin-Chat und Galerie.",
          "Die detaillierte Stimmtabelle am Termin ist zugeklappt und lässt sich öffnen.",
          "Über und unter Ja, Vielleicht, Nein und Unbekannt liegt eine Linie.",
          "Bei Ja, Vielleicht und Nein steht hinter dem Wort ein grünes Häkchen, ein oranges Fragezeichen oder ein rotes X.",
          "Auf der Titelliste kann jedes Mitglied Oh Schreck (🚨) tippen. Die Zahl zählt für alle. Noch einmal tippen nimmt es zurück.",
          "Neben den Stimmen-Kärtchen am Termin zeigt ein kleines Kreisdiagramm die Ja-Verteilung im Chor: Sopran, Alt, Tenor/Bass.",
          "Der 1:1-Chat bleibt auf diesem Gerät, wenn du ihn schließt oder die App neu öffnest. Er wird nicht auf den Server gelegt.",
          "Im Lauftext wechselt die normale Meldung mit einem Hinweis, wenn du bei kommenden Terminen noch nicht abgestimmt hast.",
          "Beim Anlegen eines Termins wählst du Datum und Uhrzeit getrennt. Die Uhrzeit ist eine Liste in Viertelstunden. Nach dem Beginn wird das Ende mit demselben Tag plus drei Stunden vorausgefüllt.",
          "Die Verwaltung kann jemanden als Planer markieren. Planer legen Termine und Umfragen in der App an und können nur selbst erstellte annehmen, absagen oder löschen. Selbst angelegte Termine zeigen Foto und Namen des Planers.",
          "Im Notenarchiv und in der Titelliste hat jede Datei Öffnen und Herunterladen. Öffnen mit gibt es nur auf Handy und iPad.",
          "Spirit of the Year ist zugeklappt und lässt sich öffnen.",
          "Im Menü heißen die Gruppenchats Chor-Chat, Band-Chat und Orchester-Chat. Der allgemeine Chat-Eintrag ist weg.",
          "Beim Öffnen erinnert der Lauftext einmal an Profilbild und Adresse, bevor die offizielle Meldung kommt.",
          "Unter Kanäle kann jedes Mitglied alle Kanäle einblenden, nicht nur die eigenen.",
          "Bei den persönlichen Daten gibt es Mitglied seit als Jahr.",
          "Läuft ein Live-Stream, steht im Lauftext, wer sendet, und dass man mit Play zuschauen kann.",
          "In der Personenübersicht der Verwaltung steht, wann jemand zuletzt verbunden war.",
          "Auf der Titelliste kann die Verwaltung Chor-Solisten setzen. Mitglieder sehen die Namen. Unter Soli steht, wie oft jemand in Konzerten ein Solo gesungen hat.",
          "Bei Songvorschlägen gibt es Daumen hoch, Neutral und Daumen runter. Noch einmal tippen nimmt die Stimme zurück.",
        ],
        en: [
          "Event chat works while a date is still in voting, not only after accept.",
          "Planners and the controller set a vote for someone else with four icons. Those votes show in blue.",
          "Dates start folded. Folding one folds them all. You still see title, time, type, the vote face, the icons, Yes/Maybe/No, and the menu.",
          "Accept, decline, and delete on a date ask first, because those cannot be undone.",
          "Hovering the alarm on a title list shows who tapped Oh God!.",
          "Deleting a date first asks: You want to delete the event?",
          "The date icons and menu sit at the top again. Planners also get a colored Accept, Decline, and Delete group there, and the same colors in the menu.",
          "Participation on a date again shows only the current face.",
          "Oh God! on a title list is a red alarm siren (🚨).",
          "Accept, cancel, and delete on your own date sit in the menu, apart from Yes, Maybe, and No.",
          "On a date, icons replace the Comments, Titles, Event chat, and Gallery buttons.",
          "The detailed vote table on a date starts folded and can be opened.",
          "A line sits above and below Yes, Maybe, No, and Unknown.",
          "Yes, Maybe, and No now show a green check, an orange question mark, or a red X after the word.",
          "On a date title list, each member can tap Oh God! (🚨). The count is shared. Tap again to take it back.",
          "Next to the voice cards on a date, a small pie chart shows the choir yes-split: soprano, alto, tenor/bass.",
          "A private chat stays on this device when you close it or reopen the app. It is not stored on the server.",
          "The news ticker alternates the official message with a reminder when you still have upcoming dates to vote on.",
          "When creating a date, pick the calendar day and the time separately. Time is a quarter-hour list. After you set the start, the end is filled with the same day plus three hours.",
          "Controllers can mark someone as a planner. Planners create dates and polls in the member app and can only accept, decline, or delete ones they created. Planner-created dates show the planner's photo and name.",
          "Music Archive and the event title list show Open and Download as icons. Open with appears only on phone and iPad.",
          "Spirit of the Year starts folded and can be opened.",
          "The menu lists Choir Chat, Band Chat, and Orchestra Chat. The generic Chat entry is gone.",
          "On each open, the news ticker once reminds you to add a profile picture and address before the official message.",
          "In Channels, any member can tick a box to see every channel, not only their own.",
          "Personal info has a Member since year.",
          "While someone is live, the news ticker says who is streaming and to press Play to join.",
          "The controller people list shows when someone last connected.",
          "Controllers can assign choir soloists on each title list. Members see the names. The Soli tab counts concert soli, not rehearsals.",
          "Song proposals have thumbs up, neutral, and thumbs down. Tap the same choice again to clear your vote.",
        ],
      },
    },
    {
      date: "2026-09-11",
      items: {
        de: [
          "Im Notenarchiv öffnen Audio und Tracks auf dem iPhone die Dateien, nicht Foto oder Video.",
          "In der Galerie hat jedes Bild und jeder Ordner eine Checkbox. Sind 1–64 Dateien gewählt, teilt Teilen sie — am Desktop heißt der Knopf Download.",
          "Die Icon-Leiste und das Menü beginnen mit Termine, Notenarchiv, Galerie, Adressbuch.",
          "Im Menü gibt es Galerie: Live, Allgemein und die Termine als Titelbilder. Live und Allgemein liegen in Monatsordnern (Monat/Jahr). Bild und Video gehen nach Live, Hochladen nach Allgemein — in einer Termin-Galerie alle drei in diesen Termin.",
          "Bei Konzerten gibt es VVK/Werbung direkt nach der Galerie: Ticket-Links, Flyer, Plakate und ein Hinweis der Verwaltung. Mitglieder sehen und öffnen sie nur.",
          "Im Videoanruf gibt es ein Vollbild-Symbol. Das Bild füllt Handy und Desktop.",
          "Im privaten Chat gibt es Anruf und Videoanruf. Die andere Person nimmt an oder lehnt ab — Ton und Bild laufen in beide Richtungen.",
          "Ein Name in der Online-Liste öffnet einen kurzen privaten Chat — auf beiden Seiten. Beim Schließen ist alles weg.",
          "Neben DE/EN steht, wie viele online sind. Darüber liegen die Namen.",
          "Unter der Tab-Leiste läuft ein Newsticker. Den Text setzt die Verwaltung unter Einstellungen.",
          "Farben folgen dem Logo: Gold für Marke und Flächen, Rose für Akzente — in Hell und Dunkel.",
          "Neben DE/EN gibt es ein Hell/Dunkel-Symbol. Dark bleibt Standard; Hell merkt sich das Gerät.",
        ],
        en: [
          "In the music archive, Audio and Tracks on iPhone open Files, not Photo or Video.",
          "Gallery pictures and folders have a checkbox. With 1–64 files selected, Share sends them to other apps. On a desktop browser the button says Download.",
          "The icon bar and menu start with Dates, Music Archive, Gallery, Address book.",
          "The menu has Gallery: Live, General, and the events as cover pictures. Live and General use month/year folders. Picture and Video go into Live, Upload into General — unless you opened an event gallery, then all three go there.",
          "Concerts have Tickets / promo right after the gallery: ticket links, flyers, posters, and a note from the controller. Members can only view and open them.",
          "A video call has a full-screen icon. The picture fills the phone or desktop.",
          "Private chats have audio and video call buttons. The other person accepts or declines, then both sides hear and see each other.",
          "A name in the online list opens a short private chat for both of you. Closing it wipes the messages.",
          "Next to DE/EN, a count shows who is online. Hover it for names.",
          "A news ticker runs under the tab bar. The controller sets the text in Settings.",
          "Colors follow the logo: gold for the brand and washes, rose for accents — in light and dark.",
          "A sun/moon control next to DE/EN switches the current dark look to a light theme and remembers it.",
        ],
      },
    },
    {
      date: "2026-09-10",
      items: {
        de: [
          "Im Chat gibt es neben dem Eingabefeld ein Senden-Symbol. Return geht weiter.",
          "Die Tab-Icons zeigen beim Überfahren den Menütext in einem kleinen Popup.",
          "Chats schließen oben rechts mit Schließen — wie Adressbuch, ohne den roten Knopf links.",
          "Oben liegt eine Tab-Leiste: Haus, Adressbuch, Archiv, Vorschlagen, Vorschläge, Add App, Kalender und Chat. Jedes Icon — und derselbe Eintrag im Menü — wechselt die Ansicht, ohne den Kopf zu überdecken.",
          "Im Notenarchiv gibt es MIDI/MusicXML als eigenen Bereich. Archiv, Songvorschläge und Adressbuch füllen das Fenster und haben unten Schließen.",
          "Kanäle, Nächster Termin und Meine Termine sind zugeklappte Paneele. Unten zeigt ein Vor/Zurück nur einen Termin, Chat füllt das Fenster und springt zur zuletzt gesehenen Nachricht.",
          "Jeder Chat hat unten einen Schließen-Knopf — Live, Chor, Band, Orchester und Termin, auch in der Verwaltung.",
          "Streamer können Kamera, eine Videodatei oder ein Fenster/Tab (z. B. YouTube) senden.",
          "Neben dem Live-Video liegt ein Live-Chat — dieselben Funktionen wie im normalen Chat, Video 75 %, Chat 25 %.",
          "Streamer können oben mit Aufnehmen einen Live-Stream vom Computer oder Handy starten. Alle können mit Abspielen zuschauen. Die Streamer-Rolle steht im Profil in der Verwaltung.",
          "Im Notenarchiv gibt es Share URL: YouTube, Spotify und ähnliche Links, ohne Datei.",
          "Oben schaltet eine rote Wecker-Glocke Benachrichtigungen — standardmäßig an. Neue Chats und Termine erscheinen als Hinweis in der App, solange sie offen ist.",
          "Das Changelog beginnt mit „New Spirit App (Author: AJP)“ und hat keinen Prompt mehr am Ende.",
          "Eigene Chat-Nachrichten haben unter dem Text ein Stift- und ein Papierkorb-Symbol — in der Mitglieder-App und in der Verwaltung.",
          "Im Menü öffnet der Avatar Foto und Info. Oben gibt es ein Chat-Symbol.",
          "Das Menü ist auf allen Bildschirmgrößen das Burger-Icon.",
          "Bei Songvorschlägen steht, wer den Song vorgeschlagen hat.",
          "Im Chat können Fotos und Videos geschickt werden — neben dem Mikrofon.",
          "Spirit of the Year zeigt Anwesenheit in Prozent: links die Führung, rechts du.",
          "Am Handy: ein Menü statt der langen Button-Leiste, Boxen über die volle Breite, Chat im Vollbild.",
          "Ehemalige können Mixer-Kanälen zugeordnet werden.",
          "Ehemalige sehen Chor-Termine und den Chor-Chat, können aber nicht abstimmen.",
          "Die Übersicht heißt „Meine Termine“. Noten-PDFs werden nicht mehr automatisch geladen.",
          "Termine ohne deine Stimme sind farblich hervorgehoben.",
          "Sprachnachrichten lassen sich mit einem Tipp löschen.",
          "Chorleiter sehen alle Rollen-Chats (Chor, Band, Orchester).",
          "Sprachnachrichten im Chat mit automatischer Abschrift. Chorleiter können Kanäle haben.",
        ],
        en: [
          "Chat has a send arrow next to the message field. Return still works.",
          "Hovering a tab icon shows the menu label in a small popup.",
          "Chats close with Close on the top right — like the address book, without the red button on the left.",
          "A tab bar on top switches the main view: home, address book, archive, propose, proposals, Add App, calendar, and chat. The menu opens the same views, and the header stays visible.",
          "The music archive has a MIDI/MusicXML slot. Archive, song proposals, and the address book fill the window and have Close at the bottom.",
          "Channels, Next Date, and My Dates start folded. A back/forward control shows one date at a time, chat fills the window, and opens at the last message you saw.",
          "Every chat has a Close button at the bottom — live, choir, band, orchestra, and event, including in the controller.",
          "Streamers can send the camera, a video file, or a window/tab (for example YouTube).",
          "A live chat sits beside the live video — the same features as the normal chat, video 75%, chat 25%.",
          "Streamers can start a live stream from a computer or phone with the Record button on top. Everyone can watch with Play. The streamer role is a checkbox on the person in the controller.",
          "The music archive has Share URL: YouTube, Spotify, and similar links, with no file upload.",
          "A red alarm-clock icon in the header toggles notifications — on by default. New chats and dates show as a banner in the app while it is open.",
          "The changelog starts with “New Spirit App (Author: AJP)” and no longer ends with a prompt.",
          "Your chat messages have a pencil and a paper-bin icon under the text — in the member app and in the controller.",
          "The avatar menu opens photo and info. The header has a chat icon.",
          "The menu is the burger icon on every screen size.",
          "Song proposals show who suggested the song.",
          "Chat can send photos and videos — the camera button sits next to the mic.",
          "Spirit of the Year shows attendance as a percent: the leader on the left, yours on the right.",
          "On phones: a menu instead of the long button row, full-width boxes, and a fullscreen chat.",
          "Alumni can be assigned to mixer channels.",
          "Alumni see choir dates and choir chat, but they cannot vote.",
          "The overview is labeled “My Dates”. Sheet-music PDFs no longer auto-load.",
          "Dates that still need your vote are highlighted.",
          "Voice notes can be deleted with one tap.",
          "Choir directors see every role chat (choir, band, orchestra).",
          "Chat voice notes with automatic transcripts. Choir directors can have channels.",
        ],
      },
    },
    {
      date: "2026-09-09",
      items: {
        de: [
          "Die Verwaltung kann Ja-Stimmen als abwesend oder krank markieren. Titellisten sind nummeriert.",
          "Event-Galerien für Fotos und Videos. Titelliste mit einem Tipp kopieren.",
          "Mitglieder können Songs und Dateien ins Archiv legen; sie warten auf Freigabe. Chat mit Emojis und Vollbild.",
          "New Spirit lässt sich auf den Home-Bildschirm legen.",
          "Adressbuch und Rolle Ehemalige. Kleidung heißt jetzt Cloth.",
          "Notenarchiv, Kanäle und Songvorschläge. Gleichstand bei Spirit of the Year wird angezeigt.",
          "Rollen- und Event-Chats mit Reaktionen.",
          "Kontakte-Tab. „Info ändern“, sobald ein Geburtstag gesetzt ist.",
          "Profilfotos, persönliche Daten, optional HTTPS.",
          "Anwesenheits-Rang und Spirit of the Year.",
          "Termin-Umfragen mit mehreren Zeitslots.",
          "Termindetails, Kommentare und eine deutsche/englische Oberfläche.",
          "New Spirit: Verfügbarkeit abstimmen.",
        ],
        en: [
          "The controller can mark Yes-voters as absent or sick. Title lists are numbered.",
          "Event galleries for photos and videos. Copy a title list with one tap.",
          "Members can add archive songs and files; they wait for approval. Chat has emojis and a full-size window.",
          "Add New Spirit to the home screen.",
          "Address book and an Alumni role. Dress options are now called cloth.",
          "Music archive, channels, and song proposals. Spirit of the Year ties are shown.",
          "Role and event chats with reactions.",
          "Contacts tab. “Modify info” after a birthday is set.",
          "Profile photos, personal info, and optional HTTPS.",
          "Choir attendance ranking and Spirit of the Year.",
          "Time polls so a date can offer several slots.",
          "Date details, comments, and a German/English UI.",
          "New Spirit availability voting.",
        ],
      },
    },
  ];

  function formatDay(iso) {
    const d = new Date(`${iso}T12:00:00`);
    return d.toLocaleDateString(I18N.locale(), { day: "numeric", month: "long", year: "numeric" });
  }

  function lines() {
    const lang = I18N.lang() === "en" ? "en" : "de";
    const out = [
      I18N.t("changelogBanner"),
      "",
      I18N.t("changelogIntro"),
      "",
    ];
    ENTRIES.forEach((entry) => {
      out.push(`## ${formatDay(entry.date)}`);
      (entry.items[lang] || entry.items.en).forEach((item) => {
        out.push(`  * ${item}`);
      });
      out.push("");
    });
    return out.join("\n").trimEnd();
  }

  function render() {
    const el = document.getElementById("changelog-body");
    if (el) el.textContent = lines();
  }

  function open() {
    render();
    document.getElementById("changelog-dialog")?.showModal();
  }

  function bind() {
    document.querySelectorAll("[data-changelog]").forEach((el) => {
      el.addEventListener("click", (e) => {
        e.preventDefault();
        open();
      });
    });
    document.getElementById("changelog-close")?.addEventListener("click", () => {
      document.getElementById("changelog-dialog")?.close();
    });
    I18N.onChange(() => {
      if (document.getElementById("changelog-dialog")?.open) render();
    });
  }

  window.Changelog = { render, open, bind };
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bind);
  } else {
    bind();
  }
})();
