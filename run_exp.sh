#!/bin/bash

# Configuración
ACTION="all"    # "compile", "run", "all".

BENCHMARK_NAME="simpleconvolution"
RUNNER_NAME="runner_star"

GPU_CONFIGS=(
    "1,2"
    # "1,2,3"
    "1,2,3,4"
    "1,2,3,4,5,6,7,8"
    "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16"
    # "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32"
    # "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64"
)

NUM_RUNS=12
EXTRA_SIM_ARGS="-trace-vis -report-all"

BENCHMARK_SRC_DIR="amd/samples/$BENCHMARK_NAME"

# --- Caso 1: Runner por Defecto ---
if [ "$RUNNER_NAME" == "runner" ]; then
    EXP_DIR="$BENCHMARK_SRC_DIR"
    OUTPUT_NAME_BASE="$BENCHMARK_NAME"
    OUTPUT_PATH="${EXP_DIR}/${OUTPUT_NAME_BASE}"
    # GENERATED_MAIN_FILE="" <- No se usa.

# --- Caso 2: Runner Experimental ---
else
    TOPOLOGY_NAME="${RUNNER_NAME//runner_/}"
    EXP_DIR="exps/$BENCHMARK_NAME/$TOPOLOGY_NAME"
    OUTPUT_NAME_BASE="${BENCHMARK_NAME}_${TOPOLOGY_NAME}"
    OUTPUT_PATH="${EXP_DIR}/${OUTPUT_NAME_BASE}"
    GENERATED_MAIN_FILE="${EXP_DIR}/main_${TOPOLOGY_NAME}.go"
fi

BASE_RUNNER_PATH="github.com/sarchlab/mgpusim/v4/amd/samples/runner"
EXP_RUNNER_PATH="github.com/sarchlab/mgpusim/v4/amd/samples/$RUNNER_NAME"


# --- Función de Compilación ---
function compile_experiment {
    echo "--- Iniciando Compilación: $OUTPUT_NAME_BASE ---"
    
    # --- 1. Asegurar directorio de salida ---
    mkdir -p "$EXP_DIR"
    
    # --- 2. Encontrar el .go de origen ---
    local source_go_file=""
    if [ -f "${BENCHMARK_SRC_DIR}/main.go" ]; then
        source_go_file="${BENCHMARK_SRC_DIR}/main.go"
    elif [ -f "${BENCHMARK_SRC_DIR}/${BENCHMARK_NAME}.go" ]; then
        source_go_file="${BENCHMARK_SRC_DIR}/${BENCHMARK_NAME}.go"
    else
        echo "Error: No se encontró 'main.go' ni '${BENCHMARK_NAME}.go' en $BENCHMARK_SRC_DIR"
        exit 1
    fi
    
    # --- 3. Lógica de compilación ---
    if [ "$RUNNER_NAME" == "runner" ]; then
        # Compilamos el archivo original en su carpeta
        go build -o "$OUTPUT_PATH" "$source_go_file"
    else
        # Debemos copiar, modificar y compilar el nuevo archivo
        cp "$source_go_file" "$GENERATED_MAIN_FILE"
        sed -i "s|${BASE_RUNNER_PATH}|${EXP_RUNNER_PATH}|" "$GENERATED_MAIN_FILE"
        go build -o "$OUTPUT_PATH" "$GENERATED_MAIN_FILE"
    fi

    if [ ! -f "$OUTPUT_PATH" ]; then
        echo "¡Error de compilación!"
        exit 1
    fi
    
    echo "Compilación finalizada: $OUTPUT_PATH"
}

# --- Función de Ejecución ---
function run_experiment {
    echo "--- Iniciando Ejecución: $OUTPUT_NAME_BASE ---"
    
    if [ ! -f "$OUTPUT_PATH" ]; then
        echo "Error: Ejecutable $OUTPUT_PATH no encontrado. Compila primero."
        exit 1
    fi
    
    # --- 1. Nos ubicamos en el directorio de salida ---
    pushd "$EXP_DIR" > /dev/null
    
    # --- 2. Ejecutamos ---
    local total_runs=0

    for gpu_list in "${GPU_CONFIGS[@]}"; do
        echo "Ejecutando con -gpus=$gpu_list ($NUM_RUNS veces)..."
        for (( i=1; i<=$NUM_RUNS; i++ )); do
            
            local sim_args="-timing $EXTRA_SIM_ARGS -gpus=$gpu_list"
            ./"$OUTPUT_NAME_BASE" $sim_args > "run_${gpu_list}_${i}.log" 2>&1
            
            total_runs=$((total_runs + 1))
        done
    done
    
    # --- 3. Volvemos al directorio original ---
    popd > /dev/null
    
    echo "Ejecución finalizada. $total_runs simulaciones completadas."
    echo "Resultados en: $EXP_DIR/"
}

# --- Lógica Principal de Control ---
case "$ACTION" in
    compile)
        compile_experiment
        ;;
    run)
        run_experiment
        ;;
    all)
        compile_experiment
        run_experiment
        ;;
    *)
        echo "Error: Acción '$ACTION' no reconocida."
        exit 1
        ;;
esac