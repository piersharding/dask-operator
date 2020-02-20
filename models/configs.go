package models

import (
	"encoding/json"

	"github.com/appscode/go/log"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	"gitlab.com/piersharding/dask-operator/utils"
	corev1 "k8s.io/api/core/v1"
)

// DaskConfigs generates the ConfigMap for
// the Dask Scheduler and Worker
func DaskConfigs(dcontext dtypes.DaskContext) (*corev1.ConfigMap, error) {

	const daskConfigs = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: dask-configs-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: dask-configs
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskController
data:
  start-jupyter-notebook.sh: |
    #!/usr/bin/env bash

    set -o errexit -o pipefail

    #source activate dask-distributed
    [ -f "${HOME}/.bash_profile" ] && source "${HOME}/.bash_profile"

    # launch the notebook - the IP address to listen on is passed in via env-var IP
    mkdir -p /app
    chmod 0777 /app
    IP=${IP:-0.0.0.0}
    NOTEBOOK_PORT=${NOTEBOOK_PORT:-8888}
    export JUPYTER_PASSWORD=${JUPYTER_PASSWORD:-changeme}
    jupyter notebook --allow-root --no-browser --ip=${IP} \
                     --port=${NOTEBOOK_PORT} \
                     --config=/jupyter_notebook_config.py /app

  jupyter_notebook_config.py: |
    import errno
    import os
    import stat
    import subprocess
    
    from jupyter_core.paths import jupyter_data_dir
    from notebook.auth import passwd
    
    # Add global to quiet error checking but I suspect that this file is defunct
    global c
    
    # Setup the Notebook to listen on all interfaces on port 8888 by default
    c.NotebookApp.ip = '*'
    c.NotebookApp.port = 8888
    c.NotebookApp.open_browser = False
    
    # Configure Networking while running under Marathon:
    if 'MARATHON_APP_ID' in os.environ:
        if 'PORT_JUPYTER' in os.environ:
            c.NotebookApp.port = int(os.environ['PORT_JUPYTER'])
    
        # Set the Access-Control-Allow-Origin header
        c.NotebookApp.allow_origin = '*'
    
        # Set Jupyter Notebook Server password to 'jupyter-<Marathon-App-Prefix>'
        # e.g., Marathon App ID '/foo/bar/app' maps to password: 'jupyter-foo-bar'
        MARATHON_APP_PREFIX = \
            '-'.join(os.environ['MARATHON_APP_ID'].split('/')[:-1])
        c.NotebookApp.password = passwd('jupyter{}'.format(MARATHON_APP_PREFIX))
    
        # Allow CORS and TLS from behind Marathon-LB/HAProxy
        # Trust X-Scheme/X-Forwarded-Proto and X-Real-Ip/X-Forwarded-For
        # Necessary if the proxy handles SSL
        if 'MARATHON_APP_LABEL_HAPROXY_GROUP' in os.environ:
            c.NotebookApp.trust_xheaders = True
    
        if 'MARATHON_APP_LABEL_HAPROXY_0_VHOST' in os.environ:
            c.NotebookApp.allow_origin = \
                'http://{}'.format(
                    os.environ['MARATHON_APP_LABEL_HAPROXY_0_VHOST']
                )
    
        if 'MARATHON_APP_LABEL_HAPROXY_0_REDIRECT_TO_HTTPS' in os.environ:
            c.NotebookApp.allow_origin = \
                'https://{}'.format(
                    os.environ['MARATHON_APP_LABEL_HAPROXY_0_VHOST']
                )
    
        # Set the Jupyter Notebook server base URL to the HAPROXY_PATH specified
        if 'MARATHON_APP_LABEL_HAPROXY_0_PATH' in os.environ:
            c.NotebookApp.base_url = \
                os.environ['MARATHON_APP_LABEL_HAPROXY_0_PATH']
    
        # Setup TLS
        if 'USE_HTTPS' in os.environ:
            SCHEDULER_TLS_CERT = os.environ.get('TLS_CERT_PATH', '/'.join(
                [os.environ['MESOS_SANDBOX'],
                 '.ssl',
                 'scheduler.crt']))
            c.NotebookApp.certfile = SCHEDULER_TLS_CERT
            SCHEDULER_TLS_KEY = os.environ.get('TLS_KEY_PATH', '/'.join(
                [os.environ['MESOS_SANDBOX'],
                 '.ssl',
                 'scheduler.key']))
            c.NotebookApp.keyfile = SCHEDULER_TLS_KEY
    
    # Set a certificate if USE_HTTPS is set to any value
    PEM_FILE = os.path.join(jupyter_data_dir(), 'notebook.pem')
    if 'USE_HTTPS' in os.environ:
        if not os.path.isfile(PEM_FILE):
            # Ensure PEM_FILE directory exists
            DIR_NAME = os.path.dirname(PEM_FILE)
            try:
                os.makedirs(DIR_NAME)
            except OSError as exc:  # Python >2.5
                if exc.errno == errno.EEXIST and os.path.isdir(DIR_NAME):
                    pass
                else:
                    raise
            # Generate a certificate if one doesn't exist on disk
            subprocess.check_call(['openssl', 'req', '-new', '-newkey', 'rsa:2048',
                                   '-days', '365', '-nodes', '-x509', '-subj',
                                   '/C=XX/ST=XX/L=XX/O=generated/CN=generated',
                                   '-keyout', PEM_FILE, '-out', PEM_FILE])
            # Restrict access to PEM_FILE
            os.chmod(PEM_FILE, stat.S_IRUSR | stat.S_IWUSR)
        c.NotebookApp.certfile = PEM_FILE
    
    # Set a password if JUPYTER_PASSWORD is set
    if 'JUPYTER_PASSWORD' in os.environ:
        c.NotebookApp.password = passwd(os.environ['JUPYTER_PASSWORD'])
        del os.environ['JUPYTER_PASSWORD']
    
    
  start-dask-scheduler.sh: |
    #!/usr/bin/env bash
    ## force upgrade of dask because 2.3.0 is buggered!
    #if [ -f /opt/conda/bin/pip ]; then
    #  /opt/conda/bin/pip install --upgrade dask
    #fi

    set -o errexit -o pipefail

    #source activate dask-distributed
    [ -f "${HOME}/.bash_profile" ] && source "${HOME}/.bash_profile"

    echo "Complete environment:"
    printenv

    if [ \( -n "${KUBERNETES_SERVICE_HOST-}" \) ]
    then

      echo ""
      echo "Command to run: "
      echo dask-scheduler --host "${DASK_HOST_NAME}" --port "${DASK_PORT_SCHEDULER}" --dashboard-address "${DASK_PORT_BOKEH}" --dashboard --dashboard-prefix "${DASK_BOKEH_APP_PREFIX}" --use-xheaders "True" --scheduler-file "dask-scheduler-connection" --local-directory "${DASK_LOCAL_DIRECTORY}"

      dask-scheduler \
        --host "${DASK_HOST_NAME}" \
        --port "${DASK_PORT_SCHEDULER}" \
        --dashboard-address "${DASK_PORT_BOKEH}" \
        --dashboard \
        --dashboard-prefix "${DASK_BOKEH_APP_PREFIX}" \
        --use-xheaders "True" \
        --scheduler-file "dask-scheduler-connection" \
        --local-directory "${DASK_LOCAL_DIRECTORY}"
    else
      dask-scheduler "$@"
    fi

  start-dask-worker.sh: |
    #!/usr/bin/env bash
    ## force upgrade of dask because 2.3.0 is buggered!
    #if [ -f /opt/conda/bin/pip ]; then
    #  /opt/conda/bin/pip install --upgrade dask
    #fi

    set -o errexit -o pipefail
    
    [ -f "${HOME}/.bash_profile" ] && source "${HOME}/.bash_profile"
    #source activate dask-distributed
    
    echo "Complete environment:"
    printenv
    
    if [ \( -n "${KUBERNETES_SERVICE_HOST-}" \) ]
    then
        echo "Dask Scheduler: ${DASK_SCHEDULER}"
        NBYTES=$(python -c \
            "import os; print(''.join([str(int(float(os.environ['DASK_MEM_LIMIT']) * 0.8)), 'e6']))" \
        )
        echo "Dask Worker Memory Limit in Bytes (1 Megabyte=1e6, 1 Gigabyte=1e9): ${NBYTES}"

        NTHREADS=$(python -c \
            "import os,math; print(int(math.ceil(float(os.environ['DASK_CPU_LIMIT']))))" \
        )
        echo "Dask Worker Threads: ${NTHREADS}"
        # dask-worker --memory-limit 7516192768 --local-directory /arl/tmp --host ${IP} --bokeh --bokeh-port 8788  --nprocs 2 --nthreads 2 --reconnect "${DASK_SCHEDULER}"
        dask-worker \
            --host "${DASK_HOST_NAME}" \
            --worker-port "${DASK_PORT_WORKER}" \
            --nanny-port "${DASK_PORT_NANNY}" \
            --dashboard \
            --dashboard-address "${DASK_PORT_BOKEH}" \
            --nthreads "${NTHREADS}" \
            --nprocs "1" \
            --name "${DASK_UID}" \
            --memory-limit "${NBYTES}" \
            --local-directory "${DASK_LOCAL_DIRECTORY}" \
            --resources "${DASK_RESOURCES}" \
            --death-timeout "180" \
            "${DASK_SCHEDULER}:${DASK_PORT_SCHEDULER}"
        #dask-worker \
        #    --local-directory "${DASK_LOCAL_DIRECTORY}" \
        #    --dashboard \
        #    --dashboard-address "${DASK_PORT_BOKEH}" \
        #"${DASK_SCHEDULER}:${DASK_PORT_SCHEDULER}"            
    else
        dask-worker "$@"
        echo "Dask Scheduler: ${DASK_SCHEDULER}"
        # dask-worker --memory-limit 7516192768 --local-directory /arl/tmp --host ${IP} --bokeh --bokeh-port 8788  --nprocs 2 --nthreads 2 --reconnect "${DASK_SCHEDULER}"
    fi
    
`
	result, err := utils.ApplyTemplate(daskConfigs, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}
	configmap := &corev1.ConfigMap{}
	if err := json.Unmarshal([]byte(result), configmap); err != nil {
		return nil, err
	}
	return configmap, err
}

// DaskJobConfigs generates the ConfigMap for
// the Dask Job
func DaskJobConfigs(dcontext dtypes.DaskContext) (*corev1.ConfigMap, error) {

	const daskJobConfigs = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: daskjob-configs-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: daskjob-configs
    app.kubernetes.io/instance: "{{ .Name }}"
    app.kubernetes.io/managed-by: DaskJobController
data:
    
  app.{{ .ScriptType }}: |
{{ .ScriptContents | indent 4 }}
    
  jupyter_notebook_config.py: |
    import errno
    import os
    import stat
    import subprocess
    
    from jupyter_core.paths import jupyter_data_dir
    from notebook.auth import passwd
    
    # Add global to quiet error checking but I suspect that this file is defunct
    global c
    
    # Setup the Notebook to listen on all interfaces on port 8888 by default
    c.NotebookApp.ip = '*'
    c.NotebookApp.port = 8888
    c.NotebookApp.open_browser = False
    
    # Configure Networking while running under Marathon:
    if 'MARATHON_APP_ID' in os.environ:
        if 'PORT_JUPYTER' in os.environ:
            c.NotebookApp.port = int(os.environ['PORT_JUPYTER'])
    
        # Set the Access-Control-Allow-Origin header
        c.NotebookApp.allow_origin = '*'
    
        # Set Jupyter Notebook Server password to 'jupyter-<Marathon-App-Prefix>'
        # e.g., Marathon App ID '/foo/bar/app' maps to password: 'jupyter-foo-bar'
        MARATHON_APP_PREFIX = \
            '-'.join(os.environ['MARATHON_APP_ID'].split('/')[:-1])
        c.NotebookApp.password = passwd('jupyter{}'.format(MARATHON_APP_PREFIX))
    
        # Allow CORS and TLS from behind Marathon-LB/HAProxy
        # Trust X-Scheme/X-Forwarded-Proto and X-Real-Ip/X-Forwarded-For
        # Necessary if the proxy handles SSL
        if 'MARATHON_APP_LABEL_HAPROXY_GROUP' in os.environ:
            c.NotebookApp.trust_xheaders = True
    
        if 'MARATHON_APP_LABEL_HAPROXY_0_VHOST' in os.environ:
            c.NotebookApp.allow_origin = \
                'http://{}'.format(
                    os.environ['MARATHON_APP_LABEL_HAPROXY_0_VHOST']
                )
    
        if 'MARATHON_APP_LABEL_HAPROXY_0_REDIRECT_TO_HTTPS' in os.environ:
            c.NotebookApp.allow_origin = \
                'https://{}'.format(
                    os.environ['MARATHON_APP_LABEL_HAPROXY_0_VHOST']
                )
    
        # Set the Jupyter Notebook server base URL to the HAPROXY_PATH specified
        if 'MARATHON_APP_LABEL_HAPROXY_0_PATH' in os.environ:
            c.NotebookApp.base_url = \
                os.environ['MARATHON_APP_LABEL_HAPROXY_0_PATH']
    
        # Setup TLS
        if 'USE_HTTPS' in os.environ:
            SCHEDULER_TLS_CERT = os.environ.get('TLS_CERT_PATH', '/'.join(
                [os.environ['MESOS_SANDBOX'],
                 '.ssl',
                 'scheduler.crt']))
            c.NotebookApp.certfile = SCHEDULER_TLS_CERT
            SCHEDULER_TLS_KEY = os.environ.get('TLS_KEY_PATH', '/'.join(
                [os.environ['MESOS_SANDBOX'],
                 '.ssl',
                 'scheduler.key']))
            c.NotebookApp.keyfile = SCHEDULER_TLS_KEY
    
    # Set a certificate if USE_HTTPS is set to any value
    PEM_FILE = os.path.join(jupyter_data_dir(), 'notebook.pem')
    if 'USE_HTTPS' in os.environ:
        if not os.path.isfile(PEM_FILE):
            # Ensure PEM_FILE directory exists
            DIR_NAME = os.path.dirname(PEM_FILE)
            try:
                os.makedirs(DIR_NAME)
            except OSError as exc:  # Python >2.5
                if exc.errno == errno.EEXIST and os.path.isdir(DIR_NAME):
                    pass
                else:
                    raise
            # Generate a certificate if one doesn't exist on disk
            subprocess.check_call(['openssl', 'req', '-new', '-newkey', 'rsa:2048',
                                   '-days', '365', '-nodes', '-x509', '-subj',
                                   '/C=XX/ST=XX/L=XX/O=generated/CN=generated',
                                   '-keyout', PEM_FILE, '-out', PEM_FILE])
            # Restrict access to PEM_FILE
            os.chmod(PEM_FILE, stat.S_IRUSR | stat.S_IWUSR)
        c.NotebookApp.certfile = PEM_FILE
    
    # Set a password if JUPYTER_PASSWORD is set
    if 'JUPYTER_PASSWORD' in os.environ:
        c.NotebookApp.password = passwd(os.environ['JUPYTER_PASSWORD'])
        del os.environ['JUPYTER_PASSWORD']
    

  start-dask-job.sh: |
    #!/usr/bin/env bash

    set -o errexit -o pipefail
    
    [ -f "${HOME}/.bash_profile" ] && source "${HOME}/.bash_profile"

    export SCRIPT_TYPE="{{ .ScriptType }}"
    export MOUNTED_FILE="{{ .MountedFile }}"
    export REPORTS_DIR=${REPORTS_DIR:-/reports}

    #echo "Scheduler: ${DASK_SCHEDULER}"
    #SCHED_HOST=$(echo "${DASK_SCHEDULER}" | cut -d: -f 1)
    #echo "Scheduler host: ${SCHED_HOST}"
    #apt update && apt install -y iputils-ping
    #ping -c 2 ${SCHED_HOST} || true

    #echo "Complete environment:"
    #printenv
    #ls -l /
    echo "SCRIPT_TYPE=${SCRIPT_TYPE}, MOUNTED_FILE=${MOUNTED_FILE}"
    cd /var/tmp

    if [ "${SCRIPT_TYPE}" = "py" ]; then
      echo "Launching /app.py"
      python /app.py
    else
      if [ "${SCRIPT_TYPE}" = "ipynb" ]; then
        # launch the notebook - the IP address to listen on is passed in via env-var IP
        echo "Launching /app.ipynb"
        export JUPYTER_PASSWORD=${JUPYTER_PASSWORD:-changeme}
        TIMEOUT=${TIMEOUT:-3600}
        [ -d "${REPORTS_DIR}" ] || ( mkdir -p ${REPORTS_DIR} && chmod 777 ${REPORTS_DIR} )
        jupyter nbconvert --execute \
                          --ExecutePreprocessor.timeout=${TIMEOUT} \
                          --config=/jupyter_notebook_config.py \
                          --to html /app.ipynb \
                          --output-dir=${REPORTS_DIR}
      else
        # this is an unknown file
        echo "Launching /app.sh"
      fi
    fi

`
	result, err := utils.ApplyTemplate(daskJobConfigs, dcontext)
	if err != nil {
		log.Debugf("ApplyTemplate Error: %+v\n", err)
		return nil, err
	}
	configmap := &corev1.ConfigMap{}
	if err := json.Unmarshal([]byte(result), configmap); err != nil {
		return nil, err
	}
	return configmap, err
}
