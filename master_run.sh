#!/bin/bash

# --- Configuración ---
BENCHMARKS=("fir" "floydwarshall" "kmeans" "matrixmultiplication" "memcopy2d2" "nbody" "pagerank" "simpleconvolution")
RUNNERS=("runner" "runner_chain" "runner_clique" "runner_mesh" "runner_star")
# BENCHMARKS=("bitonicsort")
# RUNNERS=("runner_mesh")

RUN_EXP_SCRIPT="run_exp.sh"
PROCESAR_SCRIPT="procesar_benchmarks.py"

# --- Rutas ---
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROCESAR_SCRIPT_PATH="$SCRIPT_DIR/$PROCESAR_SCRIPT"

# --- Lógica de Ejecución ---
echo ">>> INICIANDO SCRIPT MAESTRO <<<"
echo "Total de combinaciones a ejecutar: $((${#BENCHMARKS[@]} * ${#RUNNERS[@]}))"
echo "------------------------------------------------------------"

for benchmark in "${BENCHMARKS[@]}"; do
    for runner in "${RUNNERS[@]}"; do
        ( # --- Inicio del subshell ---
            set -e
            echo ">>> Iniciando: Benchmark [$benchmark], Runner [$runner]"

            # --- 1. Configurar y ejecutar run_exp.sh ---
            # ('run_exp.sh' está en el mismo SCRIPT_DIR)
            sed -i "s/^BENCHMARK_NAME=.*/BENCHMARK_NAME=\"$benchmark\"/" "$SCRIPT_DIR/$RUN_EXP_SCRIPT"
            sed -i "s/^RUNNER_NAME=.*/RUNNER_NAME=\"$runner\"/" "$SCRIPT_DIR/$RUN_EXP_SCRIPT"
            "$SCRIPT_DIR/$RUN_EXP_SCRIPT"
            
            echo "--- 'run_exp.sh' finalizado para [$benchmark, $runner] ---"


            # --- 2. Configurar y ejecutar procesar_benchmarks.py ---
            dir_path=""

            case "$runner" in
                "runner")
                    dir_path="/home/kiara/PFC/mgpusim/amd/samples/$benchmark"
                    ;;
                "runner_chain")
                    dir_path="/home/kiara/PFC/mgpusim/exps/$benchmark/chain"
                    ;;
                "runner_clique")
                    dir_path="/home/kiara/PFC/mgpusim/exps/$benchmark/clique"
                    ;;
                "runner_mesh")
                    dir_path="/home/kiara/PFC/mgpusim/exps/$benchmark/mesh"
                    ;;
                "runner_star")
                    dir_path="/home/kiara/PFC/mgpusim/exps/$benchmark/star"
                    ;;
            esac
            
            echo "  Procesando directorio: $dir_path"
            
            # Ejecutar el script de Python
            python3 "$PROCESAR_SCRIPT_PATH" "$dir_path"
            
            # --- 3. LIMPIEZA DE ARCHIVOS .sqlite3 y .log ---
            echo "  Eliminando archivos .sqlite3 en: $dir_path"
            rm -f "$dir_path"/*.sqlite3

            echo "  Limpiando archivos .log en: $dir_path"
            rm -f "$dir_path"/*.log

            echo "--- Procesamiento y limpieza finalizados para [$benchmark, $runner] ---"
            echo "------------------------------------------------------------"

        ) # --- Fin del subshell ---
        
        # Comprobar si el subshell falló
        if [ $? -ne 0 ]; then
            echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            echo "!!! ATENCIÓN: Falló la combinación [$benchmark, $runner]."
            echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            echo "------------------------------------------------------------"
        fi
        
    done
done

echo ">>> TODOS LOS EXPERIMENTOS HAN FINALIZADO <<<"
