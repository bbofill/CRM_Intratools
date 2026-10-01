import os
import smtplib
import sqlite3
from datetime import date
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from dotenv import load_dotenv
import pyodbc

load_dotenv()

DB_SERVER = os.getenv('DB_SERVER', '172.16.30.20')
DB_PORT = os.getenv('DB_PORT', '1433')
DB_NAME = os.getenv('DB_NAME', 'CRM_Intranet_Pre')
DB_USER = os.getenv('DB_USER', 'intranet')
DB_PASS = os.getenv('DB_PASS')

SQLITE_PATH = os.getenv('SQLITE_PATH', '/srv/crmintratoolstest/Data/crmintratoolstest.sqlite')

SMTP_HOST = os.getenv('SMTP_HOST', '172.16.30.25')
SMTP_PORT = int(os.getenv('SMTP_PORT', 25))
EMAIL_FROM = os.getenv('EMAIL_FROM', 'services@crm.cat')
# EMAIL_TO = os.getenv('EMAIL_TO', 'crmcom@crm.cat')
#Proves:
EMAIL_TO = os.getenv('EMAIL_TO', 'blaibofill@gmail.com')

def connect_mssql():
    conn_str = (
        f"DRIVER={{ODBC Driver 18 for SQL Server}};"
        f"SERVER={DB_SERVER},{DB_PORT};"
        f"DATABASE={DB_NAME};"
        f"UID={DB_USER};"
        f"PWD={DB_PASS};"
        f"TrustServerCertificate=yes;"
    )
    return pyodbc.connect(conn_str)

def connect_sqlite():
    return sqlite3.connect(SQLITE_PATH)

def has_active_contract_in_sqlite(sqlite_conn, mssql_id, today_str):
    cursor = sqlite_conn.cursor()
    query = """
        SELECT COUNT(c.id) 
        FROM contract c
        JOIN people p ON c.people_id = p.id
        WHERE p.people_idExternal = ? 
          AND (c.end_date IS NULL OR c.end_date = '' OR c.end_date >= ?)
    """
    cursor.execute(query, (str(mssql_id), today_str))
    count = cursor.fetchone()[0]
    return count > 0

def process_and_filter_users(mssql_conn, sqlite_conn):
    mssql_cursor = mssql_conn.cursor()
    today = date.today()
    today_str = today.isoformat()

    select_query = """
        SELECT id, name, surname, Fi_Contracte 
        FROM people 
        WHERE visible_on_web = 1 
          AND Fi_Contracte IS NOT NULL
          AND Fi_Contracte < ?
    """
    mssql_cursor.execute(select_query, (today,))
    candidates = mssql_cursor.fetchall()

    if not candidates:
        return []

    verified_users = []
    update_query = "UPDATE people SET visible_on_web = 0 WHERE id = ?"

    for u in candidates:
        if not has_active_contract_in_sqlite(sqlite_conn, u.id, today_str):
            mssql_cursor.execute(update_query, (u.id,))
            verified_users.append(u)

    mssql_conn.commit()
    return verified_users

def build_email_body(users):
    worker_cards = ""
    for u in users:
        end_date_str = u.Fi_Contracte.strftime("%d/%m/%Y") if hasattr(u.Fi_Contracte, 'strftime') else str(u.Fi_Contracte)
        worker_cards += f"""
        <div style="background:#f9f9f9; border:1px solid #e8e8e8; border-radius:8px; padding:16px; margin-bottom:12px;">
          <p style="margin:4px 0;"><b>Empleat:</b> {u.name} {u.surname}</p>
          <p style="margin:4px 0;"><b>Data Fi Contracte:</b> {end_date_str}</p>
        </div>
        """

    return f"""<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color:#f7f7f7; padding:20px;">
  <div style="max-width:650px; margin:0 auto; background:white; padding:30px; border-radius:8px;
              box-shadow:0 2px 8px rgba(0,0,0,0.15); color:#333;">
    <h2 style="color:#8b0000; text-align:center; margin:0 0 16px 0;">
      Avís: Baixa de visibilitat web
    </h2>
    <p style="margin:0 0 16px 0; font-size:14px; color:#555;">
      S'ha finalitzat el contracte de les següents persones i s'ha comprovat que <b>no tenen renovacions vigents</b>. S'ha desactivat automàticament la seva visibilitat pública al portal:
    </p>
    {worker_cards}
    <div style="background:#fff7f7; border:1px solid #f0d6d6; border-radius:8px; padding:16px; margin:18px 0;">
      <p style="margin:0; color:#8b0000; font-size:14px; font-weight:bold;">
        Atenció: S'ha de desactivar manualment dins els grups de recerca.
      </p>
    </div>
    <hr style="margin-top:24px; border:none; border-top:1px solid #ddd;">
    <p style="font-size:12px; color:#aaa; text-align:center; margin:0;">
      Centre de Recerca Matemàtica - IT
    </p>
  </div>
</body>
</html>"""

def send_notification(users):
    if not users:
        return

    msg = MIMEMultipart()
    msg['Subject'] = 'CRMIntratools - Baixa de visibilitat web per contracte finalitzat'
    msg['From'] = EMAIL_FROM
    msg['To'] = EMAIL_TO

    body_html = build_email_body(users)
    msg.attach(MIMEText(body_html, 'html'))

    with smtplib.SMTP(SMTP_HOST, SMTP_PORT) as server:
        server.send_message(msg)

def main():
    mssql_conn = connect_mssql()
    sqlite_conn = connect_sqlite()
    try:
        deactivated = process_and_filter_users(mssql_conn, sqlite_conn)
        if deactivated:
            send_notification(deactivated)
            print(f"Modificados {len(deactivated)} usuarios y notificación enviada.")
        else:
            print("Ningún usuario visible cumple las condiciones para ser dado de baja hoy.")
    except Exception as e:
        print(f"Error en la ejecución: {e}")
    finally:
        mssql_conn.close()
        sqlite_conn.close()

if __name__ == '__main__':
    main()