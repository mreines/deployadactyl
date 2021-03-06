package bluegreen_test

import (
	"errors"

	"github.com/compozed/deployadactyl/config"
	. "github.com/compozed/deployadactyl/controller/deployer/bluegreen"
	I "github.com/compozed/deployadactyl/interfaces"
	"github.com/compozed/deployadactyl/logger"
	"github.com/compozed/deployadactyl/mocks"
	"github.com/compozed/deployadactyl/randomizer"
	S "github.com/compozed/deployadactyl/structs"
	"github.com/op/go-logging"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Bluegreen", func() {

	var (
		appName        string
		appPath        string
		pushOutput     string
		loginOutput    string
		pusherFactory  *mocks.PusherCreator
		pushers        []*mocks.Pusher
		log            I.Logger
		blueGreen      BlueGreen
		environment    config.Environment
		deploymentInfo S.DeploymentInfo
		response       *Buffer
		logBuffer      *Buffer
		pushError      = errors.New("push error")
		rollbackError  = errors.New("rollback error")
	)

	BeforeEach(func() {
		appName = "appName-" + randomizer.StringRunes(10)
		appPath = "appPath-" + randomizer.StringRunes(10)
		pushOutput = "pushOutput-" + randomizer.StringRunes(10)
		loginOutput = "loginOutput-" + randomizer.StringRunes(10)
		response = NewBuffer()
		logBuffer = NewBuffer()

		log = logger.DefaultLogger(logBuffer, logging.DEBUG, "test")

		environment = config.Environment{Name: randomizer.StringRunes(10)}
		environment.Foundations = []string{randomizer.StringRunes(10), randomizer.StringRunes(10)}

		deploymentInfo = S.DeploymentInfo{AppName: appName}

		pusherFactory = &mocks.PusherCreator{}
		pushers = nil
		for range environment.Foundations {
			pusher := &mocks.Pusher{}
			pushers = append(pushers, pusher)
			pusherFactory.CreatePusherCall.Returns.Pushers = append(pusherFactory.CreatePusherCall.Returns.Pushers, pusher)
			pusherFactory.CreatePusherCall.Returns.Error = append(pusherFactory.CreatePusherCall.Returns.Error, nil)
		}

		blueGreen = BlueGreen{PusherCreator: pusherFactory, Log: log}
	})

	Context("when pusher factory fails", func() {
		It("returns an error", func() {
			pusherFactory = &mocks.PusherCreator{}
			blueGreen = BlueGreen{PusherCreator: pusherFactory, Log: log}

			for i := range environment.Foundations {
				pusherFactory.CreatePusherCall.Returns.Pushers = append(pusherFactory.CreatePusherCall.Returns.Pushers, &mocks.Pusher{})

				if i != 0 {
					pusherFactory.CreatePusherCall.Returns.Error = append(pusherFactory.CreatePusherCall.Returns.Error, errors.New("push creator failed"))
				}
			}

			err := blueGreen.Push(environment, appPath, deploymentInfo, response)

			Expect(err).To(MatchError("push creator failed"))
		})
	})

	Context("when a login command fails", func() {
		It("not start a deployment", func() {
			for i, pusher := range pushers {
				pusher.LoginCall.Write.Output = loginOutput

				if i == 0 {
					pusher.LoginCall.Returns.Error = errors.New(loginOutput)
				}
			}

			err := blueGreen.Push(environment, appPath, deploymentInfo, response)
			Expect(err).To(MatchError(LoginError{[]error{errors.New(loginOutput)}}))

			for i, pusher := range pushers {
				Expect(pusher.LoginCall.Received.FoundationURL).To(Equal(environment.Foundations[i]))
				Expect(pusher.LoginCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			}

			for range environment.Foundations {
				Eventually(response).Should(Say(loginOutput))
			}
		})
	})

	Context("when all push commands are successful", func() {
		It("can push an app to a single foundation", func() {
			By("setting a single foundation")
			var (
				foundationURL = "foundationURL-" + randomizer.StringRunes(10)
				pusher        = &mocks.Pusher{}
				pusherFactory = &mocks.PusherCreator{}
			)

			environment.Foundations = []string{foundationURL}

			pushers = nil
			pushers = append(pushers, pusher)

			pusherFactory.CreatePusherCall.Returns.Pushers = append(pusherFactory.CreatePusherCall.Returns.Pushers, pusher)
			pusherFactory.CreatePusherCall.Returns.Error = append(pusherFactory.CreatePusherCall.Returns.Error, nil)

			pusher.LoginCall.Write.Output = loginOutput
			pusher.PushCall.Write.Output = pushOutput

			blueGreen = BlueGreen{PusherCreator: pusherFactory, Log: log}

			Expect(blueGreen.Push(environment, appPath, deploymentInfo, response)).To(Succeed())

			Expect(pusher.LoginCall.Received.FoundationURL).To(Equal(foundationURL))
			Expect(pusher.LoginCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
			Expect(pusher.PushCall.Received.AppPath).To(Equal(appPath))
			Expect(pusher.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			Expect(pusher.FinishPushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))

			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
		})

		It("can push an app to multiple foundations", func() {
			By("setting up multiple foundations")
			environment.Foundations = []string{randomizer.StringRunes(10), randomizer.StringRunes(10)}

			for _, pusher := range pushers {
				pusher.LoginCall.Write.Output = loginOutput
				pusher.PushCall.Write.Output = pushOutput
			}

			Expect(blueGreen.Push(environment, appPath, deploymentInfo, response)).To(Succeed())

			for i, pusher := range pushers {
				Expect(pusher.LoginCall.Received.FoundationURL).To(Equal(environment.Foundations[i]))
				Expect(pusher.LoginCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
				Expect(pusher.PushCall.Received.AppPath).To(Equal(appPath))
				Expect(pusher.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.FinishPushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
			}

			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
		})

		Context("when deleting the venerable fails", func() {
			It("logs an error", func() {
				var (
					foundationURL = "foundationURL-" + randomizer.StringRunes(10)
					pusher        = &mocks.Pusher{}
					pusherFactory = &mocks.PusherCreator{}
				)

				environment.Foundations = []string{foundationURL}
				pushers = nil
				pushers = append(pushers, pusher)

				pusherFactory.CreatePusherCall.Returns.Pushers = append(pusherFactory.CreatePusherCall.Returns.Pushers, pusher)
				pusherFactory.CreatePusherCall.Returns.Error = append(pusherFactory.CreatePusherCall.Returns.Error, nil)

				pusher.FinishPushCall.Returns.Error = errors.New("finish push error")

				blueGreen = BlueGreen{PusherCreator: pusherFactory, Log: log}

				err := blueGreen.Push(environment, appPath, deploymentInfo, response)
				Expect(err).To(MatchError(FinishPushError{[]error{errors.New("finish push error")}}))

				Eventually(logBuffer).Should(Say("finish push error"))
			})
		})
	})

	Context("when pushing to multiple foundations", func() {
		It("checks if the app exists on each foundation", func() {
			environment.Foundations = []string{randomizer.StringRunes(10), randomizer.StringRunes(10), randomizer.StringRunes(10), randomizer.StringRunes(10)}

			for range environment.Foundations {
				pusher := &mocks.Pusher{}
				pushers = append(pushers, pusher)
				pusherFactory.CreatePusherCall.Returns.Pushers = append(pusherFactory.CreatePusherCall.Returns.Pushers, pusher)
				pusherFactory.CreatePusherCall.Returns.Error = append(pusherFactory.CreatePusherCall.Returns.Error, nil)
			}

			Expect(blueGreen.Push(environment, appPath, deploymentInfo, response)).To(Succeed())

			for i := range environment.Foundations {
				Expect(pushers[i].ExistsCall.Received.AppName).To(Equal(appName))
			}
		})
	})

	Context("when app-venerable already exists on Cloud Foundry", func() {
		It("should delete venerable instances before push", func() {
			Expect(blueGreen.Push(environment, appPath, deploymentInfo, response)).To(Succeed())

			for _, pusher := range pushers {
				Expect(pusher.FinishPushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
			}
		})

		Context("when finish push fails", func() {
			It("returns and logs an error", func() {
				finishPushError := errors.New("finish push error")
				pushers[0].FinishPushCall.Returns.Error = finishPushError

				err := blueGreen.Push(environment, appPath, deploymentInfo, response)
				Expect(err).To(MatchError(FinishPushError{[]error{finishPushError}}))

				Eventually(logBuffer).Should(Say("finish push error"))
			})
		})
	})

	Context("when at least one push command is unsuccessful", func() {
		It("should rollback all recent pushes and print Cloud Foundry logs", func() {

			for i, pusher := range pushers {
				pusher.LoginCall.Write.Output = loginOutput
				pusher.PushCall.Write.Output = pushOutput

				if i != 0 {
					pusher.PushCall.Returns.Error = pushError
				}
			}

			err := blueGreen.Push(environment, appPath, deploymentInfo, response)
			Expect(err).To(MatchError(PushError{[]error{pushError}}))

			for i, pusher := range pushers {
				Expect(pusher.LoginCall.Received.FoundationURL).To(Equal(environment.Foundations[i]))
				Expect(pusher.LoginCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
				Expect(pusher.PushCall.Received.AppPath).To(Equal(appPath))
				Expect(pusher.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.RollbackCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			}

			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
		})

		Context("when rollback fails", func() {
			It("logs an error", func() {
				pushers[0].PushCall.Returns.Error = pushError
				pushers[0].RollbackCall.Returns.Error = rollbackError

				err := blueGreen.Push(environment, appPath, deploymentInfo, response)
				Expect(err).To(MatchError(RollbackError{[]error{pushError}, []error{rollbackError}}))

				Eventually(logBuffer).Should(Say("rollback error"))
			})
		})

		It("should not rollback any pushes on the first deploy when first deploy rollback is disabled", func() {
			for _, pusher := range pushers {
				pusher.LoginCall.Write.Output = loginOutput
				pusher.PushCall.Write.Output = pushOutput
				pusher.PushCall.Returns.Error = pushError
			}

			err := blueGreen.Push(environment, appPath, deploymentInfo, response)
			Expect(err).To(MatchError(PushError{[]error{pushError, pushError}}))

			for i, pusher := range pushers {
				Expect(pusher.LoginCall.Received.FoundationURL).To(Equal(environment.Foundations[i]))
				Expect(pusher.LoginCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.ExistsCall.Received.AppName).To(Equal(deploymentInfo.AppName))
				Expect(pusher.PushCall.Received.AppPath).To(Equal(appPath))
				Expect(pusher.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(pusher.RollbackCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			}

			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
			Eventually(response).Should(Say(loginOutput))
			Eventually(response).Should(Say(pushOutput))
		})
	})
})
