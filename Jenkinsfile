@Library(['pipeline-framework', 'pipeline-toolbox']) _
import com.amadeus.swb.confluence.ConfluenceHelper
import com.amadeus.swb.StringTools

// the container id that is running the Jenkins
JENKINS_CONTAINER_ID = ""

// which folder we have the Helm chart?
chartFolder = 'deployment'

// the version of the application after the build
BUILD_VERSION = ""
RELEASE_CHANNEL=""

String bitbucketCredentials  = 'IZ_USER'
helmCredentials = 'IZ_USER'

commitMessage = ""

// which folder we have the source code?
srcFolder = '.'

// the URL of the docker repository
String DOCKER_REPOSITORY_URL
String DOCKER_REPOSITORY_NAME

DEFAULT_LOGGING_JENKINS_IMAGE_VERSION = "1.5.2"
DEFAULT_GIT_CLIFF_VERSION = "2.3.0"
DEFAULT_PANDOC_VERSION = "3.2"

// get the slug of this repository
def repoSlug() {
    return scm.getUserRemoteConfigs()[0].getUrl().split('/')[-1].replaceAll('.git','')
}

// get the docker image name
def dockerImage() {
    return repoSlug() + "/" + repoSlug()
}

// based on if this build belongs to a PR or not return the URL of docker repository accordingly
// if it is a PR, we will use the dev registry, otherwise, we will use the prd registry
def getDockerRepositoryURL() {
    if (isPullRequest() && !params.ONLY_USE_PRD_DOCKER_REGISTRY) {
        return context.DOCKER.DEDICATED_ARTIFACTORY_NAME_DEV + "." + context.DOCKER.DEDICATED_BASE_DOMAIN
    }

    return context.DOCKER.DEDICATED_ARTIFACTORY_NAME_PRD + "." + context.DOCKER.DEDICATED_BASE_DOMAIN
}

// based on if this build belongs to a PR or not return the docker repository accordingly
// if it is a PR, we will use the dev registry, otherwise, we will use the prd registry
def getDockerRepositoryName() {
    if (isPullRequest() && !params.ONLY_USE_PRD_DOCKER_REGISTRY) {
        return context.DOCKER.DEDICATED_ARTIFACTORY_NAME_DEV
    }
    
    return context.DOCKER.DEDICATED_ARTIFACTORY_NAME_PRD
}

// get the value of the attribute in the chart.yaml file
String getChartAttribute(String attribute, String chartFilePath) {
    return sh (
        script: "grep -E '^${attribute}:' ${chartFilePath} | cut -d' ' -f2",
        returnStdout: true
    ).trim()
}

// set the value of the attribute in the chart.yaml file
def setChartAttribute(String attribute, String value, String chartFilePath) {
    // set value of the attribute in the chart.yaml file
    // the line has the format: ${attribute}: <old_value>, will be replaced to ${attribute}: ${value}
    sh (
        script: "sed -i 's/^${attribute}: .*/${attribute}: ${value}/' ${chartFilePath}",
        returnStdout: true)
    
    // check if the value is updated
    def updatedValue = getChartAttribute('version', chartFilePath)
    if (value != updatedValue) {
        error("Failed to update ${attribute} to ${chartReleaseVersion}, got updated to ${updatedVersion}")
    }
}

// return the last non-merge commit message
def getCommitMessage() {
    return sh (script: "git rev-parse --verify HEAD~1 > /dev/null 2>&1 && git log --pretty=%s --no-merges -1 || git log --pretty=%B \$(git rev-list --no-merges -n 1 HEAD)", returnStdout: true).trim()
}

// install commitlint
def installCommitLint() {
     sh (
        script: """
            curl -sL https://github.com/conventionalcommit/commitlint/releases/download/v0.10.1/commitlint_0.10.1_Linux_x86_64.tar.gz -o commitlint.tar.gz
            tar -xvzf commitlint.tar.gz
            """,
        label: 'Install commitlint')
}

def installGitCliff() {
    def GIT_CLIFF_VERSION = env.GIT_CLIFF_VERSION
    sh "curl -SL https://github.com/orhun/git-cliff/releases/download/v${GIT_CLIFF_VERSION}/git-cliff-${GIT_CLIFF_VERSION}-x86_64-unknown-linux-musl.tar.gz -o ./git-cliff.tar.gz"
    sh "tar -xvzf git-cliff.tar.gz"
    sh "git config --global --add safe.directory `pwd`"
}

// Lints the commit message using commitlint.
def lintCommitMessage(commitMessage) {
    def stdout = ""
    def rc = sh (
            script: """
                # check with commitlint
                echo "${commitMessage}" | ./commitlint lint > /tmp/commitlint.log 2>&1 
                """, 
            returnStatus: true,
            label: 'Lint commit message'
        )

    stdout = readFile '/tmp/commitlint.log'
    return [stdout, rc]
}

// run the semantic release
// as we use something really "AMADEUS" with Jenkins (with artifactory...), 
// this step will only check the commit message, calculate the next version, and tag the commit (if needed)
def runSemanticRelease(boolean dryRun = false) {
    def GIT_CREDENTIALS = "${URLEncoder.encode(GIT_CREDENTIAL_USR)}:${URLEncoder.encode(GIT_CREDENTIAL_PSW)}"

    withCredentials([usernamePassword(credentialsId: helmCredentials, passwordVariable: "REGISTRY_USERNAME", usernameVariable: "REGISTRY_PASSWORD")]) {
        sh """
            git config --global --add safe.directory `pwd`

            export GIT_CREDENTIALS="${GIT_CREDENTIALS}"

            # for Helm
            export REGISTRY_HOST=${context.HELM.ARTIFACTORY_BASE_URL}/${context.HELM.ARTIFACTORY_NAME}

            npx semantic-release ${dryRun ? '--dry-run' : ''}
        """
    }
}


// construct the the URL of the repository (which we see in the browser) from the git URL
String getRepositoryURL() {
    def originalUrl = "${scm.getUserRemoteConfigs()[0].getUrl()}"

    // Define the regex pattern to match the dynamic parts of the URL
    def pattern = ~/https:\/\/(.*)\/git\/scm\/(.*?)\/(.*?)\.git/

    // Define the replacement pattern, using regex groups to insert the dynamic parts
    def replacement = 'https://$1/git/projects/$2/repos/$3'

    // Use regex to replace the URL
    def newUrl = originalUrl.replaceAll(pattern, replacement)

    return newUrl
}

// return the confluence page id from the parameters if it is defined, otherwise, return from the context
def getConfluencePageId() {
    if (params.CONFLUENCE_PAGE_ID != "") {
        return params.CONFLUENCE_PAGE_ID
    } else {
        return context.CONFLUENCE_PAGE_ID
    }
}

// Create sonar properties file if it doesn't exist
def createSonarProperties() {
    if (!fileExists('sonar-project.properties')) {
        def repoKey = repoSlug()
        def projectName = getProjectName()
        writeFile file: 'sonar-project.properties', text: """
# ID to identify this project in Sonar
sonar.projectKey=${repoKey}
# Common name to group it into a portfolio
sonar.projectName=${projectName}
# Specify all files to parse
sonar.sources=.
# Exclude unit tests from Sonar analyse
sonar.exclusions=**/*_test.go
"""
    }
}

// Create the commit lint file if it doesn't exist
def createCommitLintFile() {
    if (!fileExists('.commitlint.yaml')) {
        def repoKey = repoSlug()
        def projectName = getProjectName()
        writeFile file: '.commitlint.yaml', text: """
version: v0.9.0
formatter: default
rules:
  - type-enum
severity:
  default: error
settings:
  type-enum:
    argument:
      - feat
      - fix
      - docs
      - style
      - refactor
      - perf
      - test
      - build
      - ci
      - chore
      - revert

"""
    }
}


// return true if it is not a pull request and the target branch is a real release branch
// otherwise, return false
def isProdRelease() {
    return !isPullRequest() && (RELEASE_CHANNEL == "stable" ||  /([0-9])+(\.(([0-9])+|x)*(\.x)*)*/.matches(env.BRANCH_NAME))
}

// return true it is is not a pull request and the target branch is a beta release branch
def isBetaRelease() {
    return !isPullRequest() && RELEASE_CHANNEL == "beta"
}

// check if semantic release allows to release or not
// thanks to a step in .releaserc.yaml, we will create a file .VERSION in the workspace
// with some useful information if the release is allowed
def canRelease() {
    // this file is created through the semantic release
    // more information in .releaserc.yaml
    return fileExists(".VERSION")
}

// update the status of the Pull Request to "NEEDS_WORK" or "APPROVED"
def markPullRequest(Map options=[:]) {
    String credentialsId = options.credentialsId ?: getEnv('BB_CRED_ID', 'IZ_USER')
    String branchPattern = options.branchPattern ?: ".*"
    String status = options.status ?: ""
    if(isPullRequest() && com.amadeus.swb.StringTools.matchesStrict(env.CHANGE_TARGET, branchPattern)) {
        withCredentials([usernamePassword(credentialsId: credentialsId, passwordVariable: "PWD_ROBOT_APPROVER", usernameVariable: "ROBOT_APPROVER")]) {
            String robotUser = options.user ?: ROBOT_APPROVER
            String BBAPI_URL = "$BITBUCKET_URL/rest/api/1.0"
            def getUser = httpWrapperRequest contentType: 'APPLICATION_JSON', authentication: credentialsId, url: "$BBAPI_URL/users/$robotUser"
            def bbUser=getUser.getContent()
            def approved = (status == "APPROVED")
            def approveQuery = [ contentType: 'APPLICATION_JSON',
                                authentication: credentialsId ,
                                httpMode: 'PUT',
                                requestBody: """{"user": $bbUser, "role":"REVIEWER", "approved": ${approved}, "status":"${status}"}""",
                                url: "$BBAPI_URL/projects/${getEnv('BITBUCKET_PROJECT')}/repos/${getEnv('BITBUCKET_REPOSITORY')}/pull-requests/$CHANGE_ID/participants/$robotUser" ]
            httpWrapperRequest(approveQuery)
        }
    }
}

// create a temp folder, execute the closure, and delete the folder after the closure is executed
def usingTempDir(path, closure) {
    def dirExisted = fileExists(path)
    dir(path) {
        closure()
        if(!dirExisted) {
            // deleteDir()
        }
    }
}


def setRemoteCredential(def credentialsId = null) {
    withCredentials([[$class          : 'UsernamePasswordMultiBinding',
                      credentialsId   : StringTools.parseString(credentialsId, 'IZ_USER'),
                      usernameVariable: 'SWB_GIT_USER',
                      passwordVariable: 'SWB_GIT_PASSWORD']]) {
        String remoteURL = sh(script: 'git config --get remote.origin.url', returnStdout: true).trim()
        String url = remoteURL.replaceAll(/\s+$/, "").replaceAll(/^\s+/, "")
        if(isUnix()) {
            url = url.replaceFirst(url.contains("@") ? /(http[s]?):\/\/[^@]*@/ : /(http[s]?):\/\//, '$1://\\${SWB_GIT_USER}:\\${SWB_GIT_PASSWORD}@')
        } else {
            url = url.replaceFirst(url.contains("@") ? /(http[s]?):\/\/[^@]*@/ : /(http[s]?):\/\//, '$1://\\%SWB_GIT_USER\\%:\\%SWB_GIT_PASSWORD\\%@')
        }
        sh "git remote set-url origin $url"
    }
}

def findAllChartFolders(parentFolder) {
    def globs = [
        "${parentFolder}/Chart.yaml",
        "${parentFolder}/*/Chart.yaml",
        "${parentFolder}/*/*/Chart.yaml"
    ]

    def chartFolders = []
    for (glob in globs) {
        echo "Searching for charts in ${glob}"
    
        def files = findFiles(glob: glob)

        for (file in files) {    
            def chartFilePath = file.getPath()
            def chartDir = chartFilePath.substring(0, chartFilePath.lastIndexOf('/Chart.yaml'))
            chartFolders.add(chartDir)
        }
    }
    return chartFolders
}

def doActionInAllFolders(folders, command, actionName) {
    echo "Do action ${actionName} in folders ${folders}"
    for (folder in folders) {
        echo " --> ${actionName} in '${folder}'"

        dir(folder) {
            // check the chart
            sh """
                ${command}
            """
        }
    }
}

def getHelmRegistryUrl() {
    return "${params.HELM_REGISTRY_URL ?: context.HELM.ARTIFACTORY_BASE_URL}"
}

// Get the project name from the repository URL
// Example: ssh://git@git.rnd.amadeus.net/splunk/test-helm-chart.git -> splunk
def getProjectName() {
    def originalUrl = "${scm.getUserRemoteConfigs()[0].getUrl()}"
    // Handle SSH style URLs: ssh://git@git.rnd.amadeus.net/splunk/test-helm-chart.git
    def matcher = originalUrl =~ /.*\/([^\/]+)\/[^\/]+\.git$/
    if (matcher.matches()) {
        return matcher[0][1]
    }
    return "splunk"
}

// Configuration function that returns merged config
def getConfig(Map userConfig = [:]) {
    def defaultConfig = [
        chartPath: 'deployment',
        examplesPath: 'usage-examples',
        failOnCheckCommitMsg: true,
        publishHelmChart: true,
        buildChangelogs: false,
        confluencePageId: "",
        runUnitTests: true,
        testExamples: true,
        helmRegistryUrl: '',
        loggingJenkinsImageVersion: DEFAULT_LOGGING_JENKINS_IMAGE_VERSION,
        pandocVersion: DEFAULT_PANDOC_VERSION,
        gitCliffVersion: DEFAULT_GIT_CLIFF_VERSION,
        logLevel: 'debug'
    ]

    return defaultConfig + userConfig
}

// Main pipeline configuration
def config = getConfig([
    chartPath: 'deployment',
    examplesPath: 'usage-examples',
    publishHelmChart: true
])

pipeline {
    agent any

    environment {
        // Basic environment variables
        DOCKER_CREDENTIAL = credentials('IZ_USER')
        GIT_CREDENTIAL = credentials('IZ_USER')
        REPOSITORY_URL = "${scm.userRemoteConfigs[0].url}"
        DOCKER_IMAGE = dockerImage()
        APP_NAME = repoSlug()
        BUILD_TIME = sh(script: 'date +"%Y%m%d.%H%M%S"', , returnStdout: true).trim()
        LOG_LEVEL = "${params.LOG_LEVEL}"

        LOGGING_JENKINS_IMAGE_VERSION = "${config.loggingJenkinsImageVersion}"
        GIT_CLIFF_VERSION = "${config.gitCliffVersion}"
        PANDOC_VERSION = "${config.pandocVersion}"
    }

    options {
        timeout(time: 1, unit: 'HOURS')
        timestamps()
        buildDiscarder(logRotator(numToKeepStr: '10'))
        skipDefaultCheckout(true)
    }

    parameters {
        booleanParam(name: 'FAIL_ON_CHECK_COMMIT_MSG', defaultValue: config.failOnCheckCommitMsg, description: 'If true, the commit message will be checked for conventional commit format')
        booleanParam(name: 'PUBLISH_DOCKER_IMAGE', defaultValue: true, description: 'If false, skip the step of building docker image and push to artifactory')
        booleanParam(name: 'ONLY_USE_PRD_DOCKER_REGISTRY', defaultValue: false, description: 'If true, the docker image will be always pushed to the prod artifactory')
        booleanParam(name: 'PUBLISH_HELM_CHART', defaultValue: config.publishHelmChart, description: 'If false, skip the step of packaging helm chart and push to artifactory')
        booleanParam(name: 'BUILD_CHANGELOGS', defaultValue: config.buildChangelogs, description: 'If true, generate the changelogs even if it is not a release branch')
        string      (name: 'CONFLUENCE_PAGE_ID', defaultValue: config.confluencePageId, description: 'The id of the parent page that will store the changelogs')
        string      (name: 'LOG_LEVEL', defaultValue: config.logLevel, description: 'Log level')
    }

    stages {
        stage('Checkout') {
            steps {
                cleanWs()
                checkout scm
                setRemoteCredential()
                sh "git fetch --tags > /dev/null 2>&1"
                echo "Building ${env.JOB_NAME}..."
            }
        }

        stage('Load environment') {
            steps {
                script {
                    context = readContext(file: "workbench.yaml")
                    JENKINS_CONTAINER_ID = sh(script: "cat /etc/hostname", returnStdout: true).trim()
                    echo "The container ID is: ${JENKINS_CONTAINER_ID}"

                    DOCKER_REPOSITORY_URL = getDockerRepositoryURL()
                    DOCKER_REPOSITORY_NAME = getDockerRepositoryName()

                    GO_VERSION = sh(script: "cat ${srcFolder}/go.mod | grep 'go ' | head -n1 | cut -d' ' -f2", returnStdout: true).trim()
                    echo "The version of Go is: ${GO_VERSION}"

                    commitMessage = getCommitMessage()
                    echo "The last non-merge commit message is: `${commitMessage}`"
                }
            }
        }

        stage("Pre-Check") {
            stages {
                stage ("Check PR branch") {
                    steps {
                        script {
                            def isPRAllowed = true
                            // if the target branch is master, we need to check if there is any regex expression pattern in context.ALLOW_PR_TO_MASTER_FROM that matches the branch name, if not, we will not allow the PR
                            if (context.ALLOW_PR_TO_MASTER_FROM != null && env.TARGET_BRANCH == "master" && 
                                !context.ALLOW_PR_TO_MASTER_FROM.any { branchPattern -> com.amadeus.swb.StringTools.matchesStrict(env.BRANCH_NAME, branchPattern) }
                            ) {
                                isPRAllowed = false
                            }

                            // only allow making Pull Request from hotfix/xxx to master
                            if (isPullRequest() && !isPRAllowed) {
                                def allowedPatterns = context.ALLOW_PR_TO_MASTER_FROM.collect { pattern -> "`${pattern}`"}.join(" or ")
                                def msg = "You can only make Pull Request from ${allowedPatterns} to `master`. Please create a new PR to branch `beta` instead"
                                markPullRequest([credentialsId: bitbucketCredentials, branchPattern: env.BRANCH_NAME, status: "NEEDS_WORK"])
                                addCommentsInPullRequest(msg, 'IZ_USER')
                                error(msg)
                            }
                        }
                    }
                } // End of Check PR branch

                stage ("Install commitlint") {
                    options { retry(2) }
                    steps {
                        script {
                            installCommitLint()
                            createCommitLintFile()
                        }
                    }
                } // End of Install commitlint

                stage ("Check commit message") {
                    steps {
                        script {
                            def (checkResult, rc) = lintCommitMessage(commitMessage)
                            if (rc != 0) {
                                def msg = "Your commit message is invalid: `` ${commitMessage} ``. \nCheck result: ${checkResult} \n\nPlease follow the conventional sematic commit https://www.conventionalcommits.org/en/v1.0.0/. \nFor example, your should use the following format: \n- `feat: <new feature>` \n- `fix: <bug fix releated to the application/chart>` \n- `build: <build system>` \n- `ci: <continuous integration>` \n- `chore: <something not related to the application/chart>` \n- `docs: <documentation>`\n - ...\n\nYou can modify your commit message by using command:\n\n ```bash\ngit commit -m \"your commit message\" --amend\n```\n\nThen, push your changes again.\nFor more information, please check [CONTRIBUTING.md](./CONTRIBUTING.md)"
                                markPullRequest([credentialsId: bitbucketCredentials, branchPattern: env.BRANCH_NAME, status: "NEEDS_WORK"])
                                
                                addCommentsInPullRequest(msg, 'IZ_USER')
                                if (params.FAIL_ON_CHECK_COMMIT_MSG) {
                                    error(msg)
                                }
                            }
                        }
                    }
                } // End of Check commit message

                stage('SonarQube') {
                    steps {
                        catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE') {
                            script {
                                checkout scm
                                docker.image('dockerhub.rnd.amadeus.net/docker-dev-rfas-nce/sonar/node_18:latest').inside {
                                    stage('Build') {
                                        sh "echo build done!"
                                    }
                                    stage('SonarQube analysis') {
                                        createSonarProperties()
                                        withSwbSonarQubeEnv() {
                                            if (SONAR_ARGS?.trim()) {
                                                sh "/sonar-scanner/bin/sonar-scanner ${SONAR_ARGS}"
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        } // End of Pre-check

        stage('Go Build & Test') {
            agent {
                docker {
                    image "golang:${GO_VERSION}-alpine"
                    args '--user root -v /var/run/docker.sock:/var/run/docker.sock'
                    reuseNode true
                }
            }
            stages {
                stage('Setup Go environment') {
                    steps {
                        script {
                            def gitAuth = "${GIT_CREDENTIAL_USR}:${GIT_CREDENTIAL_PSW}"
                            sh """
                                apk add build-base docker openrc git
                                
                                 # important to allow using private go modules
                                git config --global url."https://${gitAuth}@rndwww.nce.amadeus.net/".insteadOf "https://rndwww.nce.amadeus.net/"
                                git config --global --add safe.directory `pwd`
                                go env -w GOPRIVATE="rndwww.nce.amadeus.net/*"
                                
                                cd ${srcFolder}
                                go mod tidy

                                wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v\$(cat Makefile | grep GOLANG_CI_LINT_VERSION | cut -d":" -f2 | cut -d" " -f2 | head -n1)
                                go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@\$(cat go.mod | grep ginkgo/v2 | cut -d" " -f2)
                                go get github.com/onsi/gomega/...@\$(cat go.mod | grep gomega | cut -d" " -f2)
                            """
                        }
                    }
                }

                stage('Code analysis') {
                  steps {
                    print('Running code analysis...')
                    sh """
                      cd ${srcFolder}
                      go vet ./...
                      ./bin/golangci-lint run
                    """
                  }
                } // end of code analysis

                stage('Tests') {
                    parallel {
                        stage('Unit tests') {
                            steps {
                                script {
                                    print('Running unit tests...')
                                    sh """
                                        cd ${srcFolder}
                                        ginkgo --json-report ginkgo.report -r -cover --race --randomize-all -coverprofile=coverage.out --junit-report=report.xml -output-dir=./reports
                                        go tool cover -html=./reports/coverage.out -o ./reports/coverage.html
                                    """

                                    publishHTML (target : [allowMissing: false,
                                        alwaysLinkToLastBuild: true,
                                        keepAll: true,
                                        reportDir: "${srcFolder}/reports/",
                                        reportFiles: 'coverage.html',
                                        reportName: 'Coverage Reports',
                                        reportTitles: 'Coverage Report'])

                                    if (fileExists("${srcFolder}/myapp.prof")) {
                                        archiveFiles("./${srcFolder}", "myapp.prof",  null, [".+/.+\\.prof"], true, false )
                                    }
                                }
                            }
                        } // end of unit tests

                        stage('Integration tests') {
                            steps {
                                print('Running integration tests...')
                            // sh 'go test ./integration/test.go'
                            }
                        } // end of integration tests
                    }
                }
            }
        }

        // this step dump the next version in file .VERSION
        // you can see more information in .releaserc.yaml
        stage("Prepare release version") {
            agent {
                docker {
                    image 'docker-dev-splunk/stable/devops-tools/semantic-release-ci:latest'
                    registryUrl "https://${context.DOCKER.GLOBAL_REGISTRY.split(':')[0]}"
                    registryCredentialsId 'IZ_USER'
                    args '--user root --privileged'
                    reuseNode true
                }
            } // END of agent

            steps {
                script {
                    runSemanticRelease(true)
                }
            }
        } // end of prepare version

        stage('Fake-Release preparing for PR') {
            when {
                expression { !canRelease() && isPullRequest() }
            }
            steps {
                script {
                    def lastVersion = sh(script: "git describe --tags --abbrev=0", returnStdout: true).trim() - "v"
                    sh """
                    echo "BUILD_VERSION=${lastVersion}-pr${pullRequest.getNumber()}.${BUILD_NUMBER}" > .VERSION
                    echo "RELEASE_CHANNEL=dev" >> .VERSION
                    """
                }
            }
        } // end of fake release preparing

        stage('Release preparing') {
            when {
                expression { canRelease() }
            }
            steps {
                script {
                    withEnv(readFile(".VERSION").replaceAll("(?m)^[ \t]*\r?\n", "").split('\n') as List) {
                        BUILD_VERSION = env.BUILD_VERSION
                        RELEASE_CHANNEL = env.RELEASE_CHANNEL

                        echo "The next version is: ${BUILD_VERSION}"
                        echo "The release channel is: ${RELEASE_CHANNEL}"
                    }
                }
            }
        } // end of release preparing

        stage('Preview changelogs') {
            when {
                expression { isPullRequest() }
            }
            steps {
                script {
                    if (context.CONFLUENCE.PAGE_ID && context.CONFLUENCE.SPACE_KEY) {
                        installGitCliff()
                        // run git-cliff to generate the changelogs from unreleased commits
                        // and post as a comment in the PR
                        def changes = sh(script: "./git-cliff-${env.GIT_CLIFF_VERSION}/git-cliff --unreleased", returnStdout: true).trim()

                        // remove all texts in form of: <!-- XXX -->
                        changes = changes.replaceAll(/<!--.*?-->/, '')

                        addCommentsInPullRequest("Here are the changes of your PR which will be visible in the changelog at ${context.CONFLUENCE.URL}/spaces/${context.CONFLUENCE.SPACE_KEY}/pages/${context.CONFLUENCE.PAGE_ID}  after merging this PR :\n\n${changes}\n", 'IZ_USER')
                    }
                }
            }
        } // end of preview changelogs

        stage('Update app version') {
            when {
                expression { fileExists("${srcFolder}/cmd/version.go") }
            }
          steps {
            script {
              def GIT_COMMIT_HASH = sh (script: "git log -n 1 --pretty=format:'%H'", returnStdout: true)
              def BUILD_TIME = sh(script: 'date +"%Y%m%d.%H%M%S"', , returnStdout: true).trim()

              sh """
              cd ${srcFolder}
              echo "update app version"
              sed -i -e 's#\\(VERSION\\s\\+=\\s\\+\\)"[^"]\\+"#\\1"${BUILD_VERSION}"#g' cmd/version.go
              sed -i -e 's#\\(BUILD\\s\\+=\\s\\+\\)"[^"]\\+"#\\1"${BUILD_TIME}"#g' cmd/version.go
              sed -i -e 's#\\(COMMIT\\s\\+=\\s\\+\\)"[^"]\\+"#\\1"${GIT_COMMIT_HASH}"#g' cmd/version.go
              cat cmd/version.go
              """
            }
          }
        } // end of update app version

        stage('Publish docker image') {
            when {
                allOf {
                    expression { fileExists("${srcFolder}/Dockerfile") }
                    expression { canRelease() }
                    expression { params.PUBLISH_DOCKER_IMAGE }
                }
            }
            stages {
                stage('Build image') {
                    steps {
                        script {
                            dockerImages = [
                                ['dockertag' : "${DOCKER_REPOSITORY_URL}/${RELEASE_CHANNEL}/${DOCKER_IMAGE}:${BUILD_VERSION}", 'artifactrepo' : "${DOCKER_REPOSITORY_NAME}"],
                            ]

                            if (!isPullRequest() && env.BRANCH_NAME == "master"){
                                // if it is the merge, we can push images with the tag "latest"
                                dockerImages.add(['dockertag' : "${DOCKER_REPOSITORY_URL}/${RELEASE_CHANNEL}/${DOCKER_IMAGE}:latest", 'artifactrepo' : "${DOCKER_REPOSITORY_NAME}"])
                            }

                            def dockerBuildImgTag = ""
                            def buildArgs = " --build-arg GIT_USERNAME=${GIT_CREDENTIAL_USR} --build-arg GIT_PASSWORD=${GIT_CREDENTIAL_PSW} --build-arg GO_VERSION=${GO_VERSION} "

                            dockerImages.eachWithIndex { info, index ->
                                if (index != 0) {
                                    dockerBuildImgTag += " -t " + info["dockertag"]
                                }
                            }

                            print('Building images with command ' + dockerBuildImgTag)
                            docker.build(dockerImages[0]["dockertag"], dockerBuildImgTag + " --target production ." + buildArgs )
                        }
                    }
                } // end of build image

                stage('Validating images') {
                    // agent none
                    steps {
                        script {
                            // as the logic is tested in the unit tests & integration tests
                            // in this stage, we only validate if we can create the container from docker image properly or not
                            def dockerTag = dockerImages[0]["dockertag"]

                            def tempDirName = "${System.currentTimeMillis()}"
                            
                            usingTempDir(tempDirName) {
                                // TODO: start the container and check if it is running
                            }
                           
                            
                        }
                    }
                }

                 stage('Publish images') {
                    agent none
                    steps {
                        script {
                            uploadDockerToArtifactory(dockerImages)

                            for (int i = 0; i < dockerImages.size(); i++) {
                                def info = dockerImages[i]
                                def imageLocation = info["dockertag"].replaceAll("${DOCKER_REPOSITORY_URL}", context.DOCKER.GLOBAL_REGISTRY + "/" + "${DOCKER_REPOSITORY_NAME}".replaceAll(/-nce$/, '')).replaceAll(/amadeus\.net:5002/, 'amadeus.net')

                                sh "echo images are pushed at ${imageLocation}"
                                if (!isPullRequest() && (isProdRelease() || isBetaRelease())) {
                                    imageLocation = imageLocation.replaceAll('dockerhub.rnd.amadeus.net/', '')
                                }
                                dockerImages[i]["dockertag"] = imageLocation

                                addCommentsInPullRequest("You can use docker image: `${imageLocation}` for testing", 'IZ_USER')
                            }
                        }
                    }
                }
            } // end of publish image

        } // end of publish docker image

        // publish chart to the helm repository
        stage('Publish chart') {
            when {
                allOf {
                    expression { canRelease() }
                    expression { fileExists(chartFolder)  }
                    expression { params.PUBLISH_HELM_CHART }
                }
            }
            stages {
                stage('Update versions in chart') {
                    // get the new version and update the chart.yaml
                    steps {
                        script {
                            files = findFiles(glob: "${chartFolder}/Chart.yaml")
                            if (files.size() == 0) {
                                error("No Chart.yaml found in ${chartFolder}")
                            }

                            setChartAttribute('version', "${BUILD_VERSION}", "${chartFolder}/Chart.yaml" )

                            // update docker image in values.yaml
                            def imageLocation = dockerImages[0]["dockertag"].replaceAll("${DOCKER_REPOSITORY_URL}", context.DOCKER.GLOBAL_REGISTRY + "/" + "${DOCKER_REPOSITORY_NAME}".replaceAll(/-nce$/, '')).replaceAll(/amadeus\.net:5002/, 'amadeus.net')


                            echo "Using image ${imageLocation} in the chart"

                            // replace 'name: "<DOCKER_IMAGE>"' by the real image using `sed`
                            sh "sed -i 's@<DOCKER_IMAGE>@${imageLocation}@' ${chartFolder}/values.yaml"
                            
                            // update the chart URL in the changelogs template
                            def dockerURLWithoutVersion = imageLocation.substring(0, imageLocation.lastIndexOf(":"))
                            sh "sed -i 's@<DOCKER_URL>@${dockerURLWithoutVersion}@g' cliff.toml"

                        }
                    }
                } // end of update chart version

                stage('Package chart') {
                    agent {
                        docker {
                            image 'docker-production-splunk/stable/devops-tools/logging-oc-python:' + env.LOGGING_JENKINS_IMAGE_VERSION
                            registryUrl "https://${context.DOCKER.GLOBAL_REGISTRY.split(':')[0]}"
                            registryCredentialsId 'IZ_USER'
                            args '--user root -v /var/run/docker.sock:/var/run/docker.sock'

                            // Reuse the workspace on the agent defined at top-level of
                            // Pipeline, but run inside a container.
                            reuseNode true
                        }
                    }
                    steps {
                        script {
                            files = findFiles(glob: "${chartFolder}/Chart.yaml")
                            if (files.size() == 0) {
                                error("No Chart.yaml found in ${chartFolder}")
                            }
                            def file = files[0]
                            def chartFilePath = file.getPath()
                            echo "CHECKING '${chartFilePath}'"

                            def chartDir = chartFilePath.substring(0, chartFilePath.lastIndexOf('/Chart.yaml'))
                            def chartName = getChartAttribute('name', chartFilePath)
                            def chartVersion = getChartAttribute('version', chartFilePath)
                            def templateReleasePath = "${context.HELM.ARTIFACTORY_NAME}/${chartName}"

                            echo "DEBUG name='${chartName}', version='${chartVersion}', dir='${chartDir}', templateReleasePath='${templateReleasePath}'"

                            def isTestChart = chartName.endsWith("-test")
                            if ( ( ! isTestChart )) {

                                // update version to build version if necessary
                                def chartReleaseVersion = chartVersion
                                setChartAttribute("version", chartReleaseVersion, chartFilePath)
                                echo "releasing chart to build version '${chartReleaseVersion}'"

                                // generate package file
                                def chartPackageFile = sh(
                                    script: "cd ${chartDir} && helm package . | cut -d\":\" -f2",
                                    returnStdout: true
                                ).trim()

                                // upload new version of the chart
                                def uploadSpec = """{
                                    "files": [
                                        {"pattern": "${chartPackageFile}", "target": "${templateReleasePath}/"}
                                    ]
                                }"""
                                uploadToArtifactory(uploadSpec, "splunk-helm", chartReleaseVersion)

                                def deliveredChartPath = "${context.HELM.ARTIFACTORY_BASE_URL}/${templateReleasePath}/${chartName}-${chartVersion}.tgz"
                                if (isPullRequest()) {
                                    addCommentsInPullRequest("You can use chart: `${deliveredChartPath}` for testing", 'IZ_USER')
                                }
                                echo "Chart uploaded to `${deliveredChartPath}`"

                                // update the chart URL in the changelogs template
                                def chartURLWithoutVersion = "${context.HELM.ARTIFACTORY_BASE_URL}/${templateReleasePath}/${chartName}-"
                                sh "sed -i 's@<HELM_URL>@${chartURLWithoutVersion}@g' cliff.toml"
                            }
                        }
                    } // end of steps in package chart
                } // end of package chart
            }
        }

        stage('Semantic release') {
            agent {
                docker {
                    image 'docker-dev-splunk/stable/devops-tools/semantic-release-ci:latest'
                    registryUrl "https://${context.DOCKER.GLOBAL_REGISTRY.split(':')[0]}"
                    registryCredentialsId 'IZ_USER'
                    args '--user root --privileged'
                    reuseNode true
                }
            } // END of agent
            steps {
                script {
                    runSemanticRelease(false)
                }
            }
        } // end of semantic release

        stage("Generating changelogs") {
            when {
                allOf {
                    expression { context.CONFLUENCE.PAGE_ID && context.CONFLUENCE.SPACE_KEY }
                    anyOf {
                        expression { isProdRelease() }
                        expression { isBetaRelease() && context.CONFLUENCE.ALLOW_BETA_CHANGELOGS }
                        expression { params.BUILD_CHANGELOGS }
                    }
                }
            }
            options {
                retry(3)
            }
            steps {
                script {
                    echo "Generating changelogs..."
                    sh "git fetch --tags > /dev/null 2>&1"

                    def PANDOC_VERSION = env.PANDOC_VERSION
                    def GIT_CLIFF_VERSION = env.GIT_CLIFF_VERSION
                    
                    installGitCliff()

                    def repoURL = getRepositoryURL()
                    def commitsBaseURL = repoURL + "/commits"

                    sh "sed -i 's@{{ previous\\.commit_id }}@${commitsBaseURL}/{{ previous.commit_id }}@' cliff.toml"
                    sh "sed -i 's@{{ commit_id }}@${commitsBaseURL}/{{ commit_id }}@' cliff.toml"
                    sh "sed -i 's@{{ commit\\.id }}@${commitsBaseURL}/{{ commit.id }}@' cliff.toml"
                    sh "sed -i 's@REPO_URL@${repoURL}@g' cliff.toml"
                    sh "cat cliff.toml"
                    
                    sh "git-cliff-${GIT_CLIFF_VERSION}/git-cliff --output CHANGELOG.md && cat CHANGELOG.md"
                    writeFile file:"pandoc_template.html", text: '<html>$body$</html>'

                    sh """
                    curl -SL https://github.com/jgm/pandoc/releases/download/${PANDOC_VERSION}/pandoc-${PANDOC_VERSION}-linux-amd64.tar.gz -o ./pandoc.tar.gz
                    tar xvzf pandoc.tar.gz
                    pandoc-${PANDOC_VERSION}/bin/pandoc --standalone --template pandoc_template.html --metadata title="${APP_NAME} - Changelogs" -t html5 CHANGELOG.md -o generatedHtmlFile.html && cat generatedHtmlFile.html
                    """

                    // we must have the credentials ID `IZ_USER_CONF_SAAS` in Jenkins to access the Confluence Cloud
                    def confHelper = new ConfluenceHelper(this, context.CONFLUENCE.SPACE_KEY, context.CONFLUENCE.URL ?: ConfluenceHelper.onPremisesConfluenceUrl)
                    def title = context.APP_TITLE + "'s Change logs"
                    if (isBetaRelease()) {
                        title = context.APP_TITLE + "'s Beta Change logs"
                    }
                    confHelper.createOrUpdateDocumentationPage(title, getConfluencePageId(), 'generatedHtmlFile.html')

                }
            }

        } // end of generating changelogs


    }

    post {
        always {
            script {
                if (fileExists("${srcFolder}/reports/coverage.html")) {
                    archiveArtifacts artifacts: "${srcFolder}/reports/coverage.html", allowEmptyArchive: 'true'
                }
                if (fileExists("${srcFolder}/myapp.prof")) {
                    archiveArtifacts artifacts: "${srcFolder}/myapp.prof", allowEmptyArchive: 'true'
                }

                sh "docker ps -a --format '{{.Names}}' | grep -q '${APP_NAME}-${BUILD_VERSION}' && docker stop ${APP_NAME}-${BUILD_VERSION} || true"
            }
        }
        failure {
            script {
                if ((isProdRelease() || isBetaRelease()) && context.EMAIL.FAILURE_RECIPIENTS.size() > 0) {
                    emailext subject: "[${APP_NAME}] Build failure on $env.BRANCH_NAME",
                        body: "${env.BUILD_URL}",
                        from: 'ltc-jenkins-ci@doc.mail.amadeus.com',
                        to: context.EMAIL.FAILURE_RECIPIENTS.join(',')
                }
            }
        }
    }
}
