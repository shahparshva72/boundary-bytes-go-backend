"""Chat evaluation set: question -> ground-truth SQL.

Each truth query returns one row (name, value) for the correct first answer.
name: text that must appear in the chat's first row (player/team).
value: numeric that must appear in the first row (within tolerance), or NULL to skip.
"""

MEN = "'IPL','BBL','SA20','PSL','CPL','BPL','LPL','ILT20','MLC','NPL','NTB','SMA','SSM','CTC','HND','MSL','IPT','ETPL','MCT'"
WOMEN = "'WPL','WBBL','WSSM','WHND','WCL','WTB','CEC','WSL','WTC'"
WKT = "('caught','bowled','lbw','stumped','caught and bowled','hit wicket')"
OVER = "CAST(SPLIT_PART(d.ball,'.',1) AS int)"
BPO = "COALESCE(mi.balls_per_over,6)"
PHASE = f"CASE WHEN {OVER} < (CASE WHEN {BPO}=5 THEN 5 ELSE 6 END) THEN 'powerplay' WHEN {OVER} >= 15 THEN 'death' ELSE 'middle' END"


def top_runs(where, extra_join=""):
    return f"""SELECT d.striker, SUM(d.runs_off_bat) FROM wpl_delivery d JOIN wpl_match m USING(match_id)
        LEFT JOIN wpl_match_info mi USING(match_id) {extra_join}
        WHERE d.innings<=2 AND {where} GROUP BY 1 ORDER BY 2 DESC LIMIT 1"""


def top_wkts(where):
    return f"""SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN {WKT})
        FROM wpl_delivery d JOIN wpl_match m USING(match_id) LEFT JOIN wpl_match_info mi USING(match_id)
        WHERE d.innings<=2 AND {where} GROUP BY 1 ORDER BY 2 DESC LIMIT 1"""


def top_sixes(where):
    return f"""SELECT d.striker, COUNT(*) FILTER (WHERE d.runs_off_bat=6) FROM wpl_delivery d JOIN wpl_match m USING(match_id)
        LEFT JOIN wpl_match_info mi USING(match_id) WHERE d.innings<=2 AND {where} GROUP BY 1 ORDER BY 2 DESC LIMIT 1"""


def player_runs(name, where):
    return f"""SELECT '{name}', SUM(d.runs_off_bat) FROM wpl_delivery d JOIN wpl_match m USING(match_id)
        WHERE d.innings<=2 AND d.striker='{name}' AND {where}"""


def player_wkts(name, where):
    return f"""SELECT '{name}', COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN {WKT})
        FROM wpl_delivery d JOIN wpl_match m USING(match_id) WHERE d.innings<=2 AND d.bowler='{name}' AND {where}"""


def team_wins(where, renames=True):
    team = "CASE mi.winner WHEN 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore' WHEN 'Delhi Daredevils' THEN 'Delhi Capitals' WHEN 'Kings XI Punjab' THEN 'Punjab Kings' WHEN 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant' ELSE mi.winner END" if renames else "mi.winner"
    return f"""SELECT {team}, COUNT(*) FROM wpl_match_info mi JOIN wpl_match m USING(match_id)
        WHERE mi.winner IS NOT NULL AND {where} GROUP BY 1 ORDER BY 2 DESC LIMIT 1"""


def strike_rate(where, min_balls):
    return f"""SELECT d.striker, ROUND(SUM(d.runs_off_bat)*100.0/COUNT(*) FILTER (WHERE d.wides=0),2) FROM wpl_delivery d
        JOIN wpl_match m USING(match_id) WHERE d.innings<=2 AND {where} GROUP BY 1
        HAVING COUNT(*) FILTER (WHERE d.wides=0) >= {min_balls} ORDER BY 2 DESC LIMIT 1"""


QUESTIONS = [
    # --- existing leagues: regressions
    ("E1", "existing", "Who are the top 5 run scorers in IPL history?", top_runs("m.league='IPL'")),
    ("E2", "existing", "Top wicket takers in the WPL", top_wkts("m.league='WPL'")),
    ("E3", "existing", "Most sixes in the BBL", top_sixes("m.league='BBL'")),
    ("E4", "existing", "Top run scorers in WBBL", top_runs("m.league='WBBL'")),
    ("E5", "existing", "Most wickets in SA20", top_wkts("m.league='SA20'")),
    ("E6", "existing", "How many runs did Virat Kohli score in IPL 2016?", player_runs("V Kohli", "m.league='IPL' AND EXTRACT(year FROM m.start_date)=2016")),
    ("E7", "existing", "How many wickets has Jasprit Bumrah taken in the IPL?", player_wkts("JJ Bumrah", "m.league='IPL'")),
    ("E8", "existing", "Which team has won the most matches in the IPL?", team_wins("m.league='IPL'")),
    ("E9", "existing", "Most sixes in IPL 2023", top_sixes("m.league='IPL' AND EXTRACT(year FROM m.start_date)=2023")),
    ("E10", "existing", "Top run scorers in the BBL 2023-24 season", top_runs("m.league='BBL' AND m.season='2023/24'")),
    ("E11", "existing", "Who has scored the most runs batting at number 3 in the IPL?", top_runs("m.league='IPL' AND bp.batting_position=3", "JOIN wpl_batting_position bp ON bp.match_id=d.match_id AND bp.innings=d.innings AND bp.batting_team=d.batting_team AND bp.player_name=d.striker")),
    ("E12", "existing", "Highest strike rate in the IPL with at least 1000 balls faced", strike_rate("m.league='IPL'", 1000)),
    ("E13", "existing", "How many runs has Smriti Mandhana scored in the WPL?", player_runs("S Mandhana", "m.league='WPL'")),
    ("E14", "existing", "How many wickets has Rashid Khan taken in the IPL?", player_wkts("Rashid Khan", "m.league='IPL'")),
    ("E15", "existing", "Most wickets in the death overs in the IPL", top_wkts(f"m.league='IPL' AND {OVER} BETWEEN 15 AND 19")),
    # --- new leagues
    ("N1", "new", "Top run scorers in the PSL", top_runs("m.league='PSL'")),
    ("N2", "new", "Most wickets in the Caribbean Premier League", top_wkts("m.league='CPL'")),
    ("N3", "new", "Most sixes in the T20 Blast", top_sixes("m.league='NTB'")),
    ("N4", "new", "Top run scorers in The Hundred men's competition", top_runs("m.league='HND'")),
    ("N5", "new", "Most wickets in the Women's Super Smash", top_wkts("m.league='WSSM'")),
    ("N6", "new", "How many runs has Babar Azam scored in the PSL?", player_runs("Babar Azam", "m.league='PSL'")),
    ("N7", "new", "Which team has won the most matches in the Bangladesh Premier League?", team_wins("m.league='BPL'", renames=False)),
    ("N8", "new", "Top run scorers in the Syed Mushtaq Ali Trophy", top_runs("m.league='SMA'")),
    ("N9", "new", "Most wickets in Major League Cricket", top_wkts("m.league='MLC'")),
    ("N10", "new", "Top run scorers in the ILT20", top_runs("m.league='ILT20'")),
    ("N11", "new", "Most runs in the Lanka Premier League", top_runs("m.league='LPL'")),
    ("N12", "new", "Most wickets in the Women's Hundred", top_wkts("m.league='WHND'")),
    ("N13", "new", "Top run scorers in the Charlotte Edwards Cup", top_runs("m.league='CEC'")),
    ("N14", "new", "Most sixes in the Nepal Premier League", top_sixes("m.league='NPL'")),
    ("N15", "new", "Top run scorer in the Women's Caribbean Premier League", top_runs("m.league='WCL'")),
    # --- no league named: all leagues, men's by default
    ("A1", "all", "Who has scored the most runs in T20 franchise and domestic leagues?", top_runs(f"m.league IN ({MEN})")),
    ("A2", "all", "Who has taken the most wickets across all T20 leagues?", top_wkts(f"m.league IN ({MEN})")),
    ("A3", "all", "Most sixes across all T20 leagues", top_sixes(f"m.league IN ({MEN})")),
    ("A4", "all", "Who has the most runs in women's T20 leagues?", top_runs(f"m.league IN ({WOMEN})")),
    ("A5", "all", "Most wickets in women's T20 leagues", top_wkts(f"m.league IN ({WOMEN})")),
    ("A6", "all", "Top run scorers in 2025", top_runs(f"m.league IN ({MEN}) AND EXTRACT(year FROM m.start_date)=2025")),
    ("A7", "all", "How many runs has Kieron Pollard scored in his career?", player_runs("KA Pollard", "TRUE")),
    ("A8", "all", "How many wickets has Sunil Narine taken in total?", player_wkts("SP Narine", "TRUE")),
    ("A9", "all", "Who has hit the most sixes in women's T20 leagues?", top_sixes(f"m.league IN ({WOMEN})")),
    ("A10", "all", "Highest strike rate across all leagues with at least 3000 balls faced", strike_rate(f"m.league IN ({MEN})", 3000)),
    # --- phases and The Hundred
    ("H1", "hundred", "Most wickets in the powerplay in The Hundred men's competition", top_wkts(f"m.league='HND' AND {OVER} < 5")),
    ("H2", "hundred", "Most runs in the death overs in the T20 Blast", top_runs(f"m.league='NTB' AND {OVER} BETWEEN 15 AND 19")),
    ("H3", "hundred", "Most wickets in the powerplay across all T20 leagues", top_wkts(f"m.league IN ({MEN}) AND ({PHASE})='powerplay'")),
    ("H4", "hundred", "Most sixes in the death in The Hundred men's competition", top_sixes(f"m.league='HND' AND {OVER} >= 15")),
    ("H5", "hundred", "Highest team total in The Hundred men's competition", f"""SELECT d.batting_team, SUM(d.runs_off_bat+d.extras) FROM wpl_delivery d JOIN wpl_match m USING(match_id)
        WHERE m.league='HND' AND d.innings<=2 GROUP BY d.match_id, d.innings, 1 ORDER BY 2 DESC LIMIT 1"""),
    # --- team names shared by men's and women's leagues
    ("T1", "team", "How many matches have Mumbai Indians won?", team_wins(f"m.league IN ({MEN}) AND mi.winner='Mumbai Indians'")),
    ("T2", "team", "How many matches have Perth Scorchers won?", team_wins(f"m.league IN ({MEN}) AND mi.winner='Perth Scorchers'")),
    ("T3", "team", "How many matches have Mumbai Indians won in the WPL?", team_wins("m.league='WPL' AND mi.winner='Mumbai Indians'")),
]
