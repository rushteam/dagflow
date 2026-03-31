// Jenkins Pipeline：dash 后台服务构建、推送与部署

pipeline {
    agent any

    parameters {
        choice(
            name: 'BRANCH',
            choices: ['main', 'develop', ''],
            description: '选择要构建的分支（留空使用当前分支）'
        )
        string(
            name: 'DOCKER_REGISTRY',
            defaultValue: 'crpi-xzhos6vsva8eopgj-vpc.us-east-1.personal.cr.aliyuncs.com',
            description: '镜像仓库地址'
        )
        string(
            name: 'IMAGE_NAME',
            defaultValue: 'aippy-bigdata/dash',
            description: '镜像名称（不含 registry 和 tag）'
        )
        booleanParam(
            name: 'DEPLOY',
            defaultValue: true,
            description: '构建完成后是否自动部署到 K8s'
        )
        credentials(
            name: 'DOCKER_CREDENTIALS_ID',
            credentialType: 'com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl',
            defaultValue: 'docker-registry-credentials',
            description: 'Docker 仓库凭据（类型：Username with password）',
            required: true
        )
        credentials(
            name: 'KUBECONFIG_CREDENTIAL_ID',
            credentialType: 'org.jenkinsci.plugins.plaincredentials.FileCredentialsImpl',
            defaultValue: 'algo-k8s',
            description: 'Kubeconfig 凭据（可选，Secret file 类型）',
            required: false
        )
    }

    environment {
        BUILD_DIR = 'dash'
    }

    stages {
        stage('Checkout') {
            steps {
                script {
                    def branch = params.BRANCH?.trim() ?: env.BRANCH_NAME ?: 'main'
                    if (branch != 'main' && branch != 'develop') {
                        error("只允许在 main 或 develop 分支构建，当前: ${branch}")
                    }
                    echo ">>> 构建分支: ${branch}"
                }
                checkout scm
            }
        }

        stage('Build & Push Image') {
            steps {
                script {
                    def registry = params.DOCKER_REGISTRY?.trim()
                    def credentialsId = params.DOCKER_CREDENTIALS_ID ?: 'docker-registry-credentials'

                    if (!registry) {
                        error("DOCKER_REGISTRY 未设置")
                    }

                    def gitCommit = sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim()
                    def imageBase = "${registry}/${params.IMAGE_NAME}"
                    def imageCommit = "${imageBase}:${gitCommit}"
                    def imageLatest = "${imageBase}:latest"

                    env.FULL_IMAGE = imageCommit
                    env.LATEST_IMAGE = imageLatest

                    echo ">>> 构建镜像: ${imageCommit}"

                    withCredentials([usernamePassword(
                        credentialsId: credentialsId,
                        usernameVariable: 'DOCKER_USER',
                        passwordVariable: 'DOCKER_PASS'
                    )]) {
                        sh """
                            echo "\$DOCKER_PASS" | sudo docker login -u "\$DOCKER_USER" --password-stdin '${registry}'
                        """

                        sh """
                            set -e
                            cd ${env.BUILD_DIR}
                            sudo docker build \
                                -t '${imageCommit}' \
                                -t '${imageLatest}' \
                                .
                        """
                        echo ">>> 构建成功: ${imageCommit}"

                        sh """
                            sudo docker push '${imageCommit}'
                            sudo docker push '${imageLatest}'
                        """
                        echo ">>> 推送成功: ${imageCommit}"
                    }
                }
            }
        }

        stage('Deploy') {
            when {
                expression { return params.DEPLOY }
            }
            steps {
                script {
                    def deployEnv = (env.DEPLOY_ENV ?: '').trim().toLowerCase()
                    if (!deployEnv) {
                        deployEnv = 'test'
                    } else if (deployEnv != 'test' && deployEnv != 'prod') {
                        deployEnv = 'test'
                    }
                    env.DEPLOY_ENV_NAME = deployEnv
                }

                withKubeconfig(env.DEPLOY_ENV_NAME == 'prod' ? '' : params.KUBECONFIG_CREDENTIAL_ID) {
                    script {
                        def deployEnv = env.DEPLOY_ENV_NAME
                        def deployFile = "deploy-${deployEnv}.yaml"
                        def namespace = (deployEnv == 'prod') ? 'aippy-algo-prod' : 'algo'

                        if (!fileExists("${env.BUILD_DIR}/${deployFile}")) {
                            error("部署文件不存在: ${env.BUILD_DIR}/${deployFile}")
                        }

                        env.DEPLOY_NAMESPACE = namespace
                        env.KUBECTL_CMD = (deployEnv == 'prod') ? 'sudo kubectl' : 'kubectl'
                    }

                    withCredentials([usernamePassword(
                        credentialsId: params.DOCKER_CREDENTIALS_ID ?: 'docker-registry-credentials',
                        usernameVariable: 'DOCKER_USER',
                        passwordVariable: 'DOCKER_PASS'
                    )]) {
                        sh """
                            ${env.KUBECTL_CMD} create namespace ${env.DEPLOY_NAMESPACE} --dry-run=client -o yaml | ${env.KUBECTL_CMD} apply -f -
                            ${env.KUBECTL_CMD} -n ${env.DEPLOY_NAMESPACE} create secret docker-registry aliyun-registry-secret \
                                --docker-server='${params.DOCKER_REGISTRY}' \
                                --docker-username="\$DOCKER_USER" \
                                --docker-password="\$DOCKER_PASS" \
                                --dry-run=client -o yaml | ${env.KUBECTL_CMD} apply -f -
                        """
                    }

                    script {
                        def deployEnv = env.DEPLOY_ENV_NAME
                        def deployFile = "deploy-${deployEnv}.yaml"

                        sh """
                            set -e
                            cd ${env.BUILD_DIR}
                            sed 's|image: dash:latest|image: ${env.FULL_IMAGE}|' ${deployFile} | ${env.KUBECTL_CMD} apply -f -
                            ${env.KUBECTL_CMD} -n ${env.DEPLOY_NAMESPACE} rollout status deployment/dash --timeout=120s
                        """
                    }
                    echo ">>> 部署完成"
                }
            }
        }
    }

    post {
        success {
            echo "dash 构建部署成功: ${env.FULL_IMAGE}"
        }
        failure {
            echo "dash 构建部署失败"
        }
    }
}

def withKubeconfig(String credentialId, Closure body) {
    def normalized = credentialId?.trim()
    if (!normalized || normalized.equalsIgnoreCase('null')) {
        body()
        return
    }
    withCredentials([file(credentialsId: normalized, variable: 'KUBECONFIG')]) {
        body()
    }
}
