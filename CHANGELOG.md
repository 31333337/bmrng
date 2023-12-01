# Changelog

## [0.0.2](https://github.com/31333337/bmrng/compare/v0.0.1...v0.0.2) (2023-12-01)


### Features

* Extract logger pkg, add timestamps ([#46](https://github.com/31333337/bmrng/issues/46)) ([3ab5588](https://github.com/31333337/bmrng/commit/3ab55881a4725b59d022f2414e6ac71b677c2258))
* Init leveled logging ([#45](https://github.com/31333337/bmrng/issues/45)) ([240e8cb](https://github.com/31333337/bmrng/commit/240e8cbc5113715cbdd01273d3fff162b7a1a82c))
* Private server config ([#44](https://github.com/31333337/bmrng/issues/44)) ([e814dfe](https://github.com/31333337/bmrng/commit/e814dfefd15f9eec84756e8bf3b5a98cd42ab975))
* Runtime directory ([#42](https://github.com/31333337/bmrng/issues/42)) ([433225e](https://github.com/31333337/bmrng/commit/433225eb05417f89e519798f299e6e644af7cb7c))

## 0.0.1 (2023-11-01)


### ⚠ BREAKING CHANGES

* **gateway:** dynamic packet header size ([#10](https://github.com/31333337/bmrng/issues/10))

### Features

* **bin:** Add xtrellis/bin/test-gateway-io.sh ([9f1bc1f](https://github.com/31333337/bmrng/commit/9f1bc1f7b68481e69ccb8084cd1a92433b53cc43))
* **bin:** Add xtrellis/bin/test-gateway-pipe.sh ([70a19bf](https://github.com/31333337/bmrng/commit/70a19bfe36498585f42010f70188a36ee556bf0c))
* **docker:** Init ([cda8bc4](https://github.com/31333337/bmrng/commit/cda8bc454d5077b7033ddd823d9a292a8b2e34f5))
* **docker:** Install protocol buffers and use buf to generate code ([d4fdaf8](https://github.com/31333337/bmrng/commit/d4fdaf8d6b43832a702c1c66e3018b7d7b0470f0))
* **gateway:** Add gateway message de(serialization) with test ([148c924](https://github.com/31333337/bmrng/commit/148c9248a9f87a00047d6214d884935679731678))
* **gateway:** Add GetMaxProtocolSize, update GetMaxDataSize for Packet ([c197fb4](https://github.com/31333337/bmrng/commit/c197fb4ad4681019abcd8d20f12cfeb0fc7ceb81))
* **gateway:** Add/edit args GatwayAddr{In,Out} and remove arg GatewayMsgDir ([181ce98](https://github.com/31333337/bmrng/commit/181ce980346ebbd894612e6a41bda6e1c28d9370))
* **gateway:** Add/edit debug log output ([472e871](https://github.com/31333337/bmrng/commit/472e871981a9f9e37a0049f66f338edc3d1cd196))
* **gateway:** Debug log data I/O ([fecd5b3](https://github.com/31333337/bmrng/commit/fecd5b3bebdfa3d97e465ca4547cef31d0021864))
* **gateway:** Developing client message proxy with args ([ab3c9b7](https://github.com/31333337/bmrng/commit/ab3c9b73d5e52477e16bfff6d3dbd83ba3bcbe3b))
* **gateway:** Dynamic packet header size ([#10](https://github.com/31333337/bmrng/issues/10)) ([6069110](https://github.com/31333337/bmrng/commit/60691105e9ee2e4c2193dc64244304be615b70e6))
* **gateway:** Enque mixed packetized data messages from the mix-net ([df163c2](https://github.com/31333337/bmrng/commit/df163c2f6436ec1480ece578980f033c348441f1))
* **gateway:** Generate random stream id ([fc9ac5f](https://github.com/31333337/bmrng/commit/fc9ac5fe237c5cc47cb87100ac53ce3fc2870630))
* **gateway:** Improve build and test ([#7](https://github.com/31333337/bmrng/issues/7)) ([506ea31](https://github.com/31333337/bmrng/commit/506ea31078b4b44d966cb5f168e881e97d8f3349))
* **gateway:** Output data streams as it is available ([c7dc422](https://github.com/31333337/bmrng/commit/c7dc4229f922a33b9a69f89178265b96bf6b5dff))
* **gateway:** Put incoming proxy data into message queue ([95db73c](https://github.com/31333337/bmrng/commit/95db73c4d17c712c571ef9fa6a234fd6e6e65de0))
* **gateway:** Replace message serialization with packet protocol ([0e17a3f](https://github.com/31333337/bmrng/commit/0e17a3f501701cf6f83f0800b2fae49e02df5fae))
* **gateway:** Separate message queues for client messages in/out ([4042404](https://github.com/31333337/bmrng/commit/40424042b2b20816b848dd295e715f9493461152))
* **gateway:** Store data leaving mix-net per-stream and track stream start/end ([61b119d](https://github.com/31333337/bmrng/commit/61b119d74ce7c7f9e0e2af0493a57e1f346234c3))
* **gateway:** Use http server for data streams leaving the gateway ([590f7be](https://github.com/31333337/bmrng/commit/590f7be3353044269ba01224174166a8ff215e2c))
* Hook gateway simulator within client messages ([263e5f9](https://github.com/31333337/bmrng/commit/263e5f9cab917f99ea79242eab2c70d6577f647c))
* **install-deps:** Set -xe (enable xtrace and exit upon error) ([89f80cf](https://github.com/31333337/bmrng/commit/89f80cf53e30425e3e97dc12d63b893fd675b19f))
* **pb:** Add buf workspace `zkn`, init `gateway.proto` ([5bdeca1](https://github.com/31333337/bmrng/commit/5bdeca1254601fd5f2045735e96216e2752ae836))
* **pb:** Add initial gateway message Packet ([e4bf144](https://github.com/31333337/bmrng/commit/e4bf144ee2d4deabd74a0a5c507645032cb55cd3))
* Remote network simulation ([#19](https://github.com/31333337/bmrng/issues/19)) ([f558ef6](https://github.com/31333337/bmrng/commit/f558ef67f548243ad716e9b14e5f6b62a5314586))
* **utils:** Add time statistics tracker ([052f0b5](https://github.com/31333337/bmrng/commit/052f0b58e81ebd794ddcf1b49707dbb2ff3ac8a2))
* **xtrellis coordinator:** Wait for CTRL-C to leave servers running ([8560862](https://github.com/31333337/bmrng/commit/8560862755c6201b7b0bbdcc253caac973277bbe))
* **xtrellis:** Add --runexperiment ([a406b8d](https://github.com/31333337/bmrng/commit/a406b8d2a8b7234c31edf1847def37a52e04d8da))
* **xtrellis:** Add arg --debug to enable debug log ([715378a](https://github.com/31333337/bmrng/commit/715378aede2bbe7114c09170a318c055c2539060))
* **xtrellis:** Add arg EnableGateway, init gateway ([b77eac0](https://github.com/31333337/bmrng/commit/b77eac045d21157cab03b3cbcb6248aa819ee5eb))
* **xtrellis:** Add client gateway with message queue ([549e105](https://github.com/31333337/bmrng/commit/549e105a52638641c98fa26d7892e2e109b5582d))
* **xtrellis:** Add cmd/xtrellis ([6d7cd52](https://github.com/31333337/bmrng/commit/6d7cd524eafb3c116ba929594564ae817fcf11b5))
* **xtrellis:** Add coordinator arg RoundInterval ([de1a72d](https://github.com/31333337/bmrng/commit/de1a72db91f0f545f47651cf4e9c1c01c0c1a895))
* **xtrellis:** Add utils.DebugLog ([1ef0dc0](https://github.com/31333337/bmrng/commit/1ef0dc01e49bb3fd256cfd02b015f4ae74bf9aee))
* **xtrellis:** Check if message size is sufficient before starting gateway ([6200660](https://github.com/31333337/bmrng/commit/6200660e653a5b3de48ba6e4ce27f9d565d684ae))
* **xtrellis:** Coordinator: continually run lightning rounds until CTRL-C ([e5d55f7](https://github.com/31333337/bmrng/commit/e5d55f75cda0b29261d6984f6c0bff3457b37835))
* **xtrellis:** Launch client ([dbfadd4](https://github.com/31333337/bmrng/commit/dbfadd401abbc8acd890601d9bfe7cb3b8d7e180))
* **xtrellis:** Launch server similar to client ([cfe108c](https://github.com/31333337/bmrng/commit/cfe108c2ccd2e7d8252aec8bbce972d1aa7e017d))
* **xtrellis:** Print cumulative time stats for continual lightning rounds ([3c8413e](https://github.com/31333337/bmrng/commit/3c8413e5fa57609bb828599d0ad19e957aedbf7c))
* **xtrellis:** Run coordinator mix-net separately from experiment ([ac83abf](https://github.com/31333337/bmrng/commit/ac83abf1022f1497fd42141830fd04647f04f64e))


### Bug Fixes

* **install-deps:** Add bash shebang ([d142821](https://github.com/31333337/bmrng/commit/d1428218c2a667243a4521a14b5e487c4d457a50))
* **install-deps:** Correct string equality check within [ ([94cc3ad](https://github.com/31333337/bmrng/commit/94cc3ad4b675b29acb7ccb054ea9ab70eebcb40f))
* **xtrellis:** Launch client without failure ([07c08f9](https://github.com/31333337/bmrng/commit/07c08f9161c5b12c18c9644511331ebed1c582ac))
