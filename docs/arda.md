# Arda MUD: Game Knowledge Base

This document contains a comprehensive compilation of mechanics, lore, combat specifics, and rules of conduct extracted from historical Arda MUD logs, FAQs, and guides.

## 1. Lore & Setting
Arda MUD is deeply rooted in J.R.R. Tolkien’s Middle-earth universe. 
* **Locations Identified:** The game features well-known landmarks such as the town of Bree ("Брыль") and the iconic "Prancing Pony" tavern ("Таверна 'Гарцующий пони'"). Descriptions mention the layout, the surrounding high walls, and moats.
* **NPCs Referenced:** Barliman Butterbur ("Хозяин таверны Маслютик") greets players in the Prancing Pony.
* **Factions & Guilds:** Several player clans and alignments are prominent:
  * *Dark Brotherhood* ("Орден Темное Братство")
  * *Brotherhood of Ronins* ("Братство Ронинов")
  * *Councilors / Leaders* ("Советник", "Лидер")
  * *Illegals / Unaffiliated* ("Нелегальный")

## 2. Core Game Mechanics & System
* **Client & Connection:** Players connect via a standard Telnet protocol. While generic Telnet works, dedicated MUD clients are highly recommended for hotkeys and triggers. 
* **The "SMAUG 'Я' Artifact":** MUD output demonstrates an encoding quirk where lowercase 'я' is frequently rendered as a capital 'Я' (e.g., "поднимаетсЯ", "СветитсЯ", "длЯ"). 
* **Status Prompt (`HP / MA / MV / EXP`):** The standard prompt shows the character's vital statistics: `(802/802 619/619 304/304 11911196 |)`. This represents HP (Health), Mana, Moves (Movement/Endurance), and Experience Points / Gold.
* **Player Auras & States:** The game engine tags characters and objects with contextual flags before their names:
  * `(Белая Аура) / (СераЯ Аура) / (Красная Аура)` — indicates alignment/karma (White, Gray, Red).
  * `(В полете)` — Flying.
  * `(Плавает)` — Swimming.
  * `(Светится)` — Glowing / Light-emitting.
  * `(Волшебное)` — Magic item.
* **Equipment Slots:** Players have a detailed inventory system covering specific body parts: head, neck, body, fingers, arms, shoulders, legs, wrist, shield, wielded weapon, and held items.

## 3. Combat Mechanics
The combat system is dense with textual feedback detailing avoidance, parrying, and damage severity. 
* **Damage Text:** Text alerts indicate condition states: 
  * "абсолютно здоров" (perfectly healthy)
  * "имеет несколько царапин" (has a few scratches)
  * "серьезно поцарапан" (seriously scratched)
  * "слегка поранен" (lightly wounded)
* **Melee Combat:** Characters execute moves like parrying, dodging ("уклонились"), and sweeping strikes ("Сбивающий на землю удар").
* **Magic & Spells:** Magic spells display area-of-effect and targeted damage. Examples from the log include:
  * "Едкий фонтан" (Acid Fountain) — knocks targets back ("отбрасывает").
  * "Хоровод острых льдинок" (Dance of Sharp Ice) — an ice spell that grazes or scratches players.
  * "Стена огня" (Wall of Fire) — grazes opponents.

## 4. Rules of Conduct & Ethics ("Игровая этика")
This section establishes an unwritten code of honor enforced by the game’s community:
* **No Kill-Stealing (KS):** Do not steal kills. Intervening in someone else's fight without asking can ruin their quest progression or significantly reduce their XP gain. 
* **Ask Before Joining:** If you enter a room where another player is farming mobs, you must ask permission before attacking anything. Uninvited interference often leads to retaliation (including PK - Player Killing).
* **Respecting Corpses:** Looting another player's or their rightful kill's corpse without permission is heavily frowned upon. 
* **Spam & Begging:** Avoid spamming the chat and constantly begging higher-level players for experience ("паровозинг") or high-tier equipment.

## 5. Important Newbie Tips
* **Beware of Retaliation:** Interfering in fights can get you killed, even if you are marked as "peaceful" (e.g., higher-level players might summon an aggressive mob like a Beholder to bypass non-PK restrictions and execute you).
* **Auctions:** The game has a global auction system (e.g., "Аукцион: нарукавник ночи: выставляется первый раз за 351"), providing an economy for players to buy/trade rare loot. 
* **Environmental Interaction:** Pay attention to room descriptions. Darkness limits visibility entirely ("Темно хоть глаз выколи..."), making items like a Dwarven Lamp ("гномская лампа") essential.
