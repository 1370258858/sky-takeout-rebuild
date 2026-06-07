FROM rabbitmq:3.13-management

ARG DELAYED_MESSAGE_PLUGIN_VERSION=3.13.0
RUN apt-get update \
	&& apt-get install -y --no-install-recommends curl \
	&& curl -fsSL -o /plugins/rabbitmq_delayed_message_exchange-${DELAYED_MESSAGE_PLUGIN_VERSION}.ez \
		https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v${DELAYED_MESSAGE_PLUGIN_VERSION}/rabbitmq_delayed_message_exchange-${DELAYED_MESSAGE_PLUGIN_VERSION}.ez \
	&& rabbitmq-plugins enable --offline rabbitmq_delayed_message_exchange \
	&& rm -rf /var/lib/apt/lists/*
