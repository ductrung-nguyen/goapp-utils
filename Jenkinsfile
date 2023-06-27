@Library('pipeline-toolbox') _

def getBuildVersion() {
  if (isPullRequest()) {
    return "pr${pullRequest.getNumber()}.${BUILD_NUMBER}".toString()
    } else {
    baseVersion = '0.1'
    return newBuildVersion(baseVersion)
  }
}

pipeline {
  options {
    parallelsAlwaysFailFast()
  }

  environment {
    // the registry that is used to fetch external images
    GLOBAL_REGISTRY = 'dockerhub.rnd.amadeus.net:5002'
    BUILD_VERSION = getBuildVersion()
  }

  agent any

  stages {
    stage('Build & Test') {
        agent {
          docker {
            image GLOBAL_REGISTRY + '/golang:1.20-alpine3.17'
            registryUrl 'https://' + GLOBAL_REGISTRY
            args '--user root --privileged'

            // Reuse the workspace on the agent defined at top-level of
            // Pipeline, but run inside a container.
            reuseNode true
          }
        }
      stages {
        stage('Init environment') {
          steps {
            sh 'apk add build-base docker openrc'
            sh 'go mod tidy'
            sh 'wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.52.2'
            sh 'go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@$(cat go.mod | grep ginkgo/v2 | cut -d" " -f2)'
            sh 'go get github.com/onsi/gomega/...'
          }
        }

        stage('Code analysis') {
          steps {
            print('Running code analysis...')
            sh 'go vet ./...'
            sh './bin/golangci-lint run'
          }
        }

        stage('Test') {
          parallel {

            stage('Unit tests') {
              steps {
                print('Running unit tests...')
                // sh 'go test ./... -cover -coverprofile=coverage.out'
                sh 'ginkgo --json-report ./ginkgo.report -r --race --randomize-all -coverprofile=coverage.out --junit-report=report.xml -output-dir=./reports'
                sh 'go tool cover -html=./reports/coverage.out -o ./reports/coverage.html'
                publishHTML (target : [allowMissing: false,
                  alwaysLinkToLastBuild: true,
                  keepAll: true,
                  reportDir: './reports/',
                  reportFiles: 'coverage.html',
                  reportName: 'Coverage Reports',
                  reportTitles: 'Coverage Report'])
              // junit '**/reports/*.xml'
              }
            }

            stage('Integration tests') {
              steps {
                print('Running integration tests...')
              // sh 'go test ./integration/test.go'
              }
            }

          }

        }

        stage('Publish') {
          agent none
          steps {
            script {
              if (!isPullRequest()) {
                completeBuild("v" + BUILD_VERSION)
                completeBuild(BUILD_VERSION)
              }
            }
            script {
              sh 'echo DONE!'
            }
          }
        }
      }
    }
  }
}
