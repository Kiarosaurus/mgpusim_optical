import os
import sqlite3
import csv
import re
import random
from datetime import datetime
import sys

# --- Configuración ---

if len(sys.argv) < 2:
    print("ERROR: No se especificó un directorio.")
    sys.exit(1) # Termina el script si no hay argumento (directorio)

directorio = sys.argv[1]

# Validar que el directorio existe
if not os.path.isdir(directorio):
    print(f"ERROR: El directorio no existe: {directorio}")
    sys.exit(1)

NOMBRE_TABLA = "exec_info"

NOMBRE_ARCHIVO_SALIDA = "resultados_benchmark.csv"
RUTA_ARCHIVO_SALIDA = os.path.join(directorio, NOMBRE_ARCHIVO_SALIDA)

# --- Función de extracción de gpus (1era columna) ---
def ExtraerConteoGPUs(command_str):
    match = re.search(r'-gpus=([\d,]+)', command_str)
    if not match:
        print(f"ADVERTENCIA: No se encontró el patrón '-gpus=' en: {command_str}. Saltando.")
        return None
    
    gpus_str = match.group(1)
    
    try:
        ultimo_numero = gpus_str.split(',')[-1]
        return int(ultimo_numero)
    except (ValueError, IndexError):
        print(f"ADVERTENCIA: No se pudo parsear el N de GPUs desde: {gpus_str}. Saltando.")
        return None

# --- Lógica de extracción completa ---
print(f"Iniciando procesamiento de archivos en: {directorio}")
print(f"Buscando en la tabla: {NOMBRE_TABLA}\n")

resultados = {}

for filename in os.listdir(directorio):
    if filename.endswith(".sqlite3"):
        filepath = os.path.join(directorio, filename)
        
        try:
            conn = sqlite3.connect(filepath)
            cursor = conn.cursor()  # Para consultas .sqlite3
            
            # --- 1. Encontrar la subtabla exec_info ---
            cursor.execute("SELECT name FROM sqlite_master WHERE type='table' AND name=?;", (NOMBRE_TABLA,))
            if cursor.fetchone() is None:
                print(f"  ERROR: La tabla '{NOMBRE_TABLA}' no existe en '{filename}'. Saltando archivo.")
                conn.close()
                continue

            query = f"SELECT Property, Value FROM {NOMBRE_TABLA}"
            cursor.execute(query)
            datos_simulacion = dict(cursor.fetchall())
            conn.close() 

            # --- 2. Obtener la información del tiempo y comandos ---
            command_str = datos_simulacion.get("Command")
            start_time_str = datos_simulacion.get("Start Time")
            end_time_str = datos_simulacion.get("End Time")
            
            if not all([command_str, start_time_str, end_time_str]):
                print(f"  Advertencia: Datos incompletos en '{filename}'. Saltando.")
                continue

            gpu_key = ExtraerConteoGPUs(command_str)
            if gpu_key is None:
                continue
                
            try:
                time_format = "%Y-%m-%d %H:%M:%S.%f"
                start_dt = datetime.strptime(start_time_str[0:26], time_format)
                end_dt = datetime.strptime(end_time_str[0:26], time_format)
                total_time = (end_dt - start_dt).total_seconds()
            except ValueError as e:
                print(f"  ERROR DE FORMATO: {e}. Saltando.")
                continue
            
            if total_time < 0:
                print(f"  ADVERTENCIA: Tiempo negativo detectado ({total_time}s) en '{filename}'. IGNORADO.")
                continue

            if gpu_key not in resultados:
                resultados[gpu_key] = []
            
            resultados[gpu_key].append(total_time)

        except Exception as e:
            print(f"  ERROR al procesar '{filename}': {e}")

# --- Guardado de resultados ---
if not resultados:
    print(f"\nProcesamiento finalizado para {directorio}. LA DATA ERA INVÁLIDA.")
else:
    print(f"\nProcesamiento finalizado para {directorio}. Escribiendo CSV...")
    
    keys_ordenadas = sorted(resultados.keys())
    
    NUM_TIEMPOS_FIJOS = 10
    encabezados = ["GPUs"] + [f"Tiempo_{i+1} (s)" for i in range(NUM_TIEMPOS_FIJOS)]
    
    try:
        with open(RUTA_ARCHIVO_SALIDA, 'w', newline='', encoding='utf-8') as f:
            writer = csv.writer(f, delimiter=';')
            writer.writerow(encabezados)
            
            for gpu_key in keys_ordenadas:
                tiempos_de_esta_fila = resultados[gpu_key]
                n_tiempos_esta_fila = len(tiempos_de_esta_fila)
                
                tiempos_seleccionados = []
                
                # --- 1. Subset random de los tiempos (o no) ---
                if n_tiempos_esta_fila > NUM_TIEMPOS_FIJOS:
                    tiempos_seleccionados = random.sample(tiempos_de_esta_fila, NUM_TIEMPOS_FIJOS)
                else:
                    tiempos_seleccionados = tiempos_de_esta_fila
                
                tiempos_formateados = [f"{t:.6f}".replace('.', ',') for t in tiempos_seleccionados]
                fila = [gpu_key] + tiempos_formateados
                writer.writerow(fila)
                
        print(f"¡Éxito! Resultados guardados en: {RUTA_ARCHIVO_SALIDA}")
        
    except Exception as e:
        print(f"ERROR al escribir el CSV en {directorio}: {e}")