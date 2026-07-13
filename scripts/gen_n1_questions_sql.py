#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
解析叉车N1司机证理论考试题库 Markdown，生成 PostgreSQL 插入脚本。
- 题目与课程无关，仅插入 question 表和 knowledge_point 表
- 按题型设置难度等级: 判断题=beginner, 单选题=intermediate, 多选题=advanced
- 按题目内容关键词自动归类到知识点
用法: python gen_n1_questions_sql.py
输出: .trae/documents/insert_n1_questions.sql
"""

import re
import json
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
INPUT_FILE = PROJECT_ROOT / ".workbuddy" / "叉车N1司机证理论考试题库.md"
OUTPUT_FILE = PROJECT_ROOT / ".trae" / "documents" / "insert_n1_questions.sql"


# ===== 知识点定义（按主题分类）=====
# (知识点名称, 等级, 匹配关键词列表)
KNOWLEDGE_POINTS = [
    ("法律法规与证件管理", "beginner", [
        "特种设备", "作业证", "N1", "证书", "复审", "领证", "考证", "证件", "市场监管",
        "安全法", "报废", "监管", "持证", "培训", "年检", "违章", "转借", "涂改",
    ]),
    ("叉车结构与工作原理", "beginner", [
        "结构", "组成", "原理", "平衡重", "门架", "货叉", "液压系统", "齿轮泵", "溢流阀",
        "换向阀", "单向阀", "节流阀", "油箱", "油缸", "传动", "转向", "电气", "蓄电池",
        "额定起重量", "载荷中心", "转弯半径", "起升高度", "爬坡", "倾斜角", "帕斯卡",
        "动力", "变速箱", "驱动桥", "轮胎", "链条", "护顶架", "铭牌", "控制器",
    ]),
    ("安全操作规范", "beginner", [
        "限速", "行驶", "转弯", "倒车", "停车", "起步", "坡道", "会车", "交叉路口",
        "鸣笛", "盲区", "视线", "载货", "货叉离地", "门架后倾", "安全距离", "避让",
        "劳保", "安全帽", "安全鞋", "反光", "驾驶姿势", "安全带", "防护装置",
        "禁止", "严禁", "违章", "违规",
    ]),
    ("货物搬运与堆码", "intermediate", [
        "货物", "搬运", "堆码", "托盘", "捆绑", "重心", "偏载", "超载", "起升", "降落",
        "货叉", "堆垛", "装卸", "码放", "滑落", "倒塌", "散落", "插取", "对位",
    ]),
    ("日常检查与维护保养", "intermediate", [
        "检查", "保养", "维护", "润滑", "黄油", "液压油", "机油", "刹车油", "冷却",
        "预热", "怠速", "充电", "电池", "电解液", "蒸馏水", "滤清器", "防冻",
        "轮胎", "气压", "花纹", "磨损", "链条", "松紧", "封存", "停放", "清洁",
        "外观", "螺丝", "管路", "渗漏", "异响", "温度", "油温", "水温", "排气",
        "黑烟", "蓝烟", "白烟", "变黑", "浑浊", "变质", "更换", "加注",
    ]),
    ("故障诊断与应急处理", "advanced", [
        "故障", "失灵", "漏油", "异响", "无力", "下降", "过热", "卡顿", "跑偏",
        "发软", "抖动", "冒烟", "鼓包", "变形", "裂纹", "磨损", "损坏", "更换",
        "侧翻", "倾覆", "跳车", "灭火", "火灾", "灼伤", "碰撞", "事故", "应急",
        "处置", "报修", "停机", "泄压", "防爆", "危险", "隐患", "安全阀",
    ]),
]


def sql_escape(text: str) -> str:
    return text.replace("'", "''")


def match_knowledge_point(content: str):
    """根据题目内容匹配知识点，返回知识点名称；无匹配返回 None"""
    best_match = None
    best_score = 0
    for kp_name, _, keywords in KNOWLEDGE_POINTS:
        score = sum(1 for kw in keywords if kw in content)
        if score > best_score:
            best_score = score
            best_match = kp_name
    return best_match


def parse_true_false_questions(lines):
    questions = []
    pattern = re.compile(r'^(\d+)\.\s*(.+?)([√×])\s*$')
    for line in lines:
        line = line.strip()
        m = pattern.match(line)
        if m:
            num = int(m.group(1))
            content = m.group(2).strip()
            mark = m.group(3)
            answer = 'TRUE' if mark == '√' else 'FALSE'
            questions.append({
                'num': num,
                'content': content,
                'answer': answer,
                'type': 'true_false',
                'level': 'beginner',
            })
    return questions


def parse_single_choice_questions(lines):
    questions = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
        m = re.match(r'^(\d+)[\.、]\s*(.+)', line)
        if not m:
            continue
        num = int(m.group(1))
        rest = m.group(2).strip()

        bold_matches = list(re.finditer(r'\*\*(.+?)\*\*', rest))
        correct_letters = []
        for bm in bold_matches:
            bold_text = bm.group(1)
            letter_m = re.match(r'^([A-D])\s*[\.．、]', bold_text)
            if letter_m:
                correct_letters.append(letter_m.group(1))

        clean_rest = re.sub(r'\*\*', '', rest)
        opt_pattern = re.compile(r'([A-D])\s*[\.．、]\s*')
        opt_matches = list(opt_pattern.finditer(clean_rest))

        if len(opt_matches) < 2:
            continue

        question_text = clean_rest[:opt_matches[0].start()].strip()

        options = {}
        for i, om in enumerate(opt_matches):
            letter = om.group(1)
            start = om.end()
            if i + 1 < len(opt_matches):
                end = opt_matches[i + 1].start()
            else:
                end = len(clean_rest)
            opt_text = clean_rest[start:end].strip()
            opt_text = re.sub(r'\s+', ' ', opt_text).strip()
            options[letter] = opt_text

        if not correct_letters:
            continue

        questions.append({
            'num': num,
            'content': question_text,
            'options': options,
            'answer': correct_letters[0],
            'type': 'single_choice',
            'level': 'intermediate',
        })
    return questions


def parse_multi_choice_questions(lines):
    questions = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
        m = re.match(r'^(\d+)[\.、]\s*(.+)', line)
        if not m:
            continue
        num = int(m.group(1))
        rest = m.group(2).strip()

        bold_matches = list(re.finditer(r'\*\*(.+?)\*\*', rest))
        correct_letters = []
        for bm in bold_matches:
            bold_text = bm.group(1)
            letters = re.findall(r'([A-D])\s*[\.．、]', bold_text)
            correct_letters.extend(letters)

        clean_rest = re.sub(r'\*\*', '', rest)
        opt_pattern = re.compile(r'([A-D])\s*[\.．、]\s*')
        opt_matches = list(opt_pattern.finditer(clean_rest))

        if len(opt_matches) < 2:
            continue

        question_text = clean_rest[:opt_matches[0].start()].strip()

        options = {}
        for i, om in enumerate(opt_matches):
            letter = om.group(1)
            start = om.end()
            if i + 1 < len(opt_matches):
                end = opt_matches[i + 1].start()
            else:
                end = len(clean_rest)
            opt_text = clean_rest[start:end].strip()
            opt_text = re.sub(r'\s+', ' ', opt_text).strip()
            options[letter] = opt_text

        if not correct_letters:
            continue

        correct_letters = sorted(set(correct_letters))
        questions.append({
            'num': num,
            'content': question_text,
            'options': options,
            'answer': ','.join(correct_letters),
            'type': 'multi_choice',
            'level': 'advanced',
        })
    return questions


def generate_sql(tf_questions, sc_questions, mc_questions):
    lines = []
    lines.append("-- ============================================================")
    lines.append("-- 叉车N1司机证理论考试题库 - 数据库插入脚本")
    lines.append("-- 题库来源: .workbuddy/叉车N1司机证理论考试题库.md")
    lines.append("-- 生成工具: scripts/gen_n1_questions_sql.py")
    lines.append("-- 适配数据库: PostgreSQL (与 migrations/000001_init.up.sql 对应)")
    lines.append("--")
    lines.append("-- 说明:")
    lines.append("--   1. 题目独立于课程，仅插入 question 和 knowledge_point 表")
    lines.append("--   2. 按题型分难度: 判断题=beginner / 单选题=intermediate / 多选题=advanced")
    lines.append("--   3. 按内容关键词归类到知识点(knowledge_point)")
    lines.append("--   4. 分值: 判断题2分 / 单选题3分 / 多选题4分 (与后端 examScoreMap 一致)")
    lines.append("--")
    lines.append("-- Docker 执行方式 (容器名: forklift-pg-prod):")
    lines.append("--   docker cp .trae/documents/insert_n1_questions.sql forklift-pg-prod:/tmp/")
    lines.append("--   docker exec -it forklift-pg-prod psql -U forklift -d forklift_training -f /tmp/insert_n1_questions.sql")
    lines.append("-- ============================================================")
    lines.append("")
    lines.append("BEGIN;")
    lines.append("")

    # 1. 插入知识点
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 1. 插入知识点 (knowledge_point)")
    lines.append("--    用于题目分类，学员可按知识点专项练习")
    lines.append("-- ----------------------------------------------------------")
    lines.append("INSERT INTO knowledge_point (name, level, description, created_at) VALUES")
    kp_values = []
    for i, (name, level, keywords) in enumerate(KNOWLEDGE_POINTS):
        desc = f"包含关键词: {', '.join(keywords[:5])}..."
        vals = f"('{sql_escape(name)}', '{level}', '{sql_escape(desc)}', now())"
        if i < len(KNOWLEDGE_POINTS) - 1:
            vals += ","
        else:
            vals += ";"
        kp_values.append(vals)
    lines.extend(kp_values)
    lines.append("")

    # 获取知识点ID的CTE映射
    kp_names = [kp[0] for kp in KNOWLEDGE_POINTS]
    lines.append("-- 使用 CTE 建立知识点名称到ID的映射")
    lines.append("-- (通过子查询在插入时动态获取 knowledge_point.id)")
    lines.append("")

    # 2. 为每道题匹配知识点
    all_questions = tf_questions + sc_questions + mc_questions
    for q in all_questions:
        q['kp_name'] = match_knowledge_point(q['content'])

    # 统计知识点分布
    kp_stats = {}
    for q in all_questions:
        kp = q['kp_name'] or "未分类"
        kp_stats[kp] = kp_stats.get(kp, 0) + 1

    # 3. 插入题目
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 2. 插入题目 (question)")
    lines.append("--    题型: true_false(判断题) / single_choice(单选题) / multi_choice(多选题)")
    lines.append("--    难度: beginner(判断题) / intermediate(单选题) / advanced(多选题)")
    lines.append("--    状态: published (已发布)")
    lines.append("-- ----------------------------------------------------------")
    lines.append("")

    # 判断题
    lines.append("-- ===== 第一部分: 判断题 ({}题, 难度: beginner) =====".format(len